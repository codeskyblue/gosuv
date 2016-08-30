package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/urfave/cli"
)

var (
	Version string = "dev"
)

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
	}
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
