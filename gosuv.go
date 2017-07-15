package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"

	"github.com/equinox-io/equinox"
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
		return errors.New("json loads error: " + string(body))
	}
	if ret.Status != 0 {
		return fmt.Errorf("%v", ret.Value)
	}
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
			Name:   "start",
			Usage:  "Start program",
			Action: actionStart,
		},
		{
			Name:   "stop",
			Usage:  "Stop program",
			Action: actionStop,
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
			Name:   "edit",
			Usage:  "Edit config file",
			Action: actionEdit,
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
