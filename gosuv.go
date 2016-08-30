package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/equinox-io/equinox"
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
)

func equinoxUpdate(channel string) error {
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

	// fetch the update and apply it
	err = resp.Apply()
	if err != nil {
		return err
	}

	fmt.Printf("Updated to new version: %s!\n", resp.ReleaseVersion)
	return nil
}

func actionStartServer(c *cli.Context) error {
	if err := registerHTTPHandlers(); err != nil {
		return err
	}
	addr := c.String("address")
	if c.Bool("foreground") {
		fmt.Println("added serv: ", addr)
		log.Fatal(http.ListenAndServe(addr, nil))
	} else {
		log.Fatal("Not implement daemon mode")
	}
	return nil
}

func actionStatus(c *cli.Context) error {
	resp, err := http.Get("http://localhost:8000/api/status")
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

func actionShutdown(c *cli.Context) error {
	resp, err := http.Get("http://localhost:8000/api/shutdown")
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
	if err := registerHTTPHandlers(); err != nil {
		log.Fatal(err)
	}
	log.Println("test is successful")
	return nil
}

func actionUpdateSelf(c *cli.Context) error {
	return equinoxUpdate(c.String("channel"))
}

func main() {
	app := cli.NewApp()
	app.Name = "gosuv"
	app.Version = Version
	app.Usage = "golang port of python-supervisor"
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
					Name:  "address, addr",
					Usage: "listen address",
					Value: ":8000",
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
			Name:   "shutdown",
			Usage:  "Shutdown server",
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
			},
			Action: actionUpdateSelf,
		},
	}
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
