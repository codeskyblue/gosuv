package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/urfave/cli"
)

var (
	GitSummary string = "unknown"
)

func actionServ(c *cli.Context) error {
	fmt.Println("added serv: ", c.Args().First())

	log.Fatal(http.ListenAndServe(":8000", nil))
	return nil
}

func actionStatus(c *cli.Context) error {
	log.Println("Status")
	return nil
}

func main() {
	app := cli.NewApp()
	app.Name = "gosuv"
	app.Version = GitSummary
	app.Usage = "golang port of python-supervisor"
	app.Commands = []cli.Command{
		{
			Name:   "serv",
			Usage:  "Should only called by itself",
			Action: actionServ,
		},
		{
			Name:    "status",
			Aliases: []string{"st"},
			Usage:   "Show program status",
			Action:  actionStatus,
		},
	}
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
