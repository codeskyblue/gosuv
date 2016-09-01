package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/equinox-io/equinox"
	"github.com/goji/httpauth"
	"github.com/qiniu/log"
	"github.com/urfave/cli"
)

const appID = "app_8Gji4eEAdDx"

var (
	Version   string = "dev"
	publicKey        = []byte(`
-----BEGIN ECDSA PUBLIC KEY-----
MHYwEAYHKoZIzj0CAQYFK4EEACIDYgAEY8xsSkcFs8XXUicw3n7E77qN/vqKUQ/6
/X5aBiOVF1yTIRYRXrV3aEvJRzErvQxziT9cLxQq+BFUZqn9pISnPSf9dn0wf9kU
TxI79zIvne9UT/rDsM0BxSydwtjG00MT
-----END ECDSA PUBLIC KEY-----
`)
	cfg Configuration
)

func equinoxUpdate(channel string, skipConfirm bool) error {
	var opts equinox.Options
	if err := opts.SetPublicKeyPEM(publicKey); err != nil {
		return err
	}
	opts.Channel = channel

	// check for the update
	resp, err := equinox.Check(appID, opts)
	switch {
	case err == equinox.NotAvailableErr:
		fmt.Println("No update available, already at the latest version!")
		return nil
	case err != nil:
		fmt.Println("Update failed:", err)
		return err
	}

	fmt.Println("New version available!")
	fmt.Println("Version:", resp.ReleaseVersion)
	fmt.Println("Name:", resp.ReleaseTitle)
	fmt.Println("Details:", resp.ReleaseDescription)

	if !skipConfirm {
		fmt.Printf("Would you like to update [y/n]? ")
		if !askForConfirmation() {
			return nil
		}
	}
	//fmt.Printf("New version available: %s downloading ... \n", resp.ReleaseVersion)
	// fetch the update and apply it
	err = resp.Apply()
	if err != nil {
		return err
	}

	fmt.Printf("Updated to new version: %s!\n", resp.ReleaseVersion)
	return nil
}

func actionStartServer(c *cli.Context) error {
	hdlr, err := newSupervisorHandler()
	if err != nil {
		log.Fatal(err)
	}
	auth := cfg.Server.HttpAuth
	if auth.Enabled {
		hdlr = httpauth.SimpleBasicAuth(auth.User, auth.Password)(hdlr)
	}
	http.Handle("/", hdlr)

	addr := cfg.Server.Addr
	if c.Bool("foreground") {
		log.Printf("server listen on %v", addr)
		log.Fatal(http.ListenAndServe(addr, nil))
	} else {
		if checkServerStatus() == nil {
			fmt.Println("server is already running")
			return nil
		}
		logPath := filepath.Join(defaultConfigDir, "gosuv.log")
		logFd, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("create file %s failed: %v", logPath, err)
		}
		cmd := exec.Command(os.Args[0], "start-server", "-f")
		cmd.Stdout = logFd
		cmd.Stderr = logFd
		err = cmd.Start()
		if err != nil {
			log.Fatal(err)
		}
		select {
		case err = <-GoFunc(cmd.Wait):
			log.Fatalf("server started failed, %v", err)
		case <-time.After(200 * time.Millisecond):
			showAddr := addr
			if strings.HasPrefix(addr, ":") {
				showAddr = "0.0.0.0" + addr
			}
			fmt.Printf("server started, listening on %s\n", showAddr)
		}
	}
	return nil
}

func checkServerStatus() error {
	resp, err := http.Get(cfg.Client.ServerURL + "/api/status")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var ret JSONResponse
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return err
	}
	if ret.Status != 0 {
		return fmt.Errorf("%v", ret.Value)
	}
	return nil
}

func actionStatus(c *cli.Context) error {
	err := checkServerStatus()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println("Server is running, OK.")
	}
	return nil
}

func actionShutdown(c *cli.Context) error {
	restart := c.Bool("restart")
	if restart {
		log.Fatal("Restart not implemented.")
	}
	resp, err := http.Post(cfg.Client.ServerURL+"/api/shutdown", "application/x-www-form-urlencoded", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var ret JSONResponse
	err = json.Unmarshal(body, &ret)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ret.Value)
	return nil
}

func actionReload(c *cli.Context) error {
	resp, err := http.Post(cfg.Client.ServerURL+"/api/reload", "application/x-www-form-urlencoded", nil)
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Fatal(err)
	}
	var ret JSONResponse
	err = json.Unmarshal(body, &ret)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ret.Value)
	return nil
}

func actionConfigTest(c *cli.Context) error {
	if _, err := newSupervisorHandler(); err != nil {
		log.Fatal(err)
	}
	log.Println("test is successful")
	return nil
}

func actionUpdateSelf(c *cli.Context) error {
	return equinoxUpdate(c.String("channel"), c.Bool("yes"))
}

func actionVersion(c *cli.Context) error {
	fmt.Printf("gosuv version %s\n", Version)
	return nil
}

func main() {
	var defaultConfigPath = filepath.Join(defaultConfigDir, "config.yml")

	app := cli.NewApp()
	app.Name = "gosuv"
	app.Version = Version
	app.Usage = "golang port of python-supervisor"
	app.Before = func(c *cli.Context) error {
		var err error
		cfgPath := c.GlobalString("conf")
		cfg, err = readConf(cfgPath)
		if err != nil {
			log.Fatal(err)
		}
		return nil
	}
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "codeskyblue",
			Email: "codeskyblue@gmail.com",
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "conf, c",
			Usage: "config file",
			Value: defaultConfigPath,
		},
	}
	app.Commands = []cli.Command{
		{
			Name:  "start-server",
			Usage: "Start supervisor and run in background",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "foreground, f",
					Usage: "start in foreground",
				},
				cli.StringFlag{
					Name:  "conf, c",
					Usage: "config file",
					Value: defaultConfigPath,
				},
			},
			Action: actionStartServer,
		},
		{
			Name:    "status",
			Aliases: []string{"st"},
			Usage:   "Show program status",
			Action:  actionStatus,
		},
		{
			Name:   "reload",
			Usage:  "Reload config file",
			Action: actionReload,
		},
		{
			Name:  "shutdown",
			Usage: "Shutdown server",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "restart, r",
					Usage: "restart server(todo)",
				},
			},
			Action: actionShutdown,
		},
		{
			Name:    "conftest",
			Aliases: []string{"t"},
			Usage:   "Test if config file is valid",
			Action:  actionConfigTest,
		},
		{
			Name:  "update-self",
			Usage: "Update gosuv itself",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "channel, c",
					Usage: "update channel name, stable or dev",
					Value: "stable",
				},
				cli.BoolFlag{
					Name:  "yes, y",
					Usage: "Do not promote to confirm",
				},
			},
			Action: actionUpdateSelf,
		},
		{
			Name:    "version",
			Usage:   "Show version",
			Aliases: []string{"v"},
			Action:  actionVersion,
		},
	}
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
