package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/codegangsta/cli"
)

func MkdirIfNoExists(dir string) error {
	dir = os.ExpandEnv(dir)
	if _, err := os.Stat(dir); err != nil {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func StatusAction(ctx *cli.Context) {
	println("status todo")
}

func AddAction(ctx *cli.Context) {
	name := ctx.String("name")
	fmt.Printf("program: %s has been added\n", strconv.Quote(name))
}

func StopAction(ctx *cli.Context) {
}

var app *cli.App

func init() {
	app = cli.NewApp()
	app.Version = "0.0.1"
	app.Name = "gosuv"
	app.Usage = "supervisor your program"
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:   "port",
			Value:  17422,
			Usage:  "server listen port",
			EnvVar: "GOSUV_SERVER_PORT",
		},
		cli.StringFlag{
			Name:   "host",
			Value:  "127.0.0.1",
			Usage:  "server listen host",
			EnvVar: "GOSUV_SERVER_HOST",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "status",
			Usage:  "show program status",
			Action: StatusAction,
		},
		{
			Name:  "add",
			Usage: "add to running list",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "program name",
				},
			},
			Action: AddAction,
		},
		{
			Name:   "stop",
			Usage:  "stop running program",
			Action: StopAction,
		},
	}
}

const (
	GOSUV_HOME   = "$HOME/.gosuv"
	GOSUV_CONFIG = "gosuv.json"
)

func main() {
	MkdirIfNoExists(GOSUV_HOME)
	app.RunAndExitOnError()
}
