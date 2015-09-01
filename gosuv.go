package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"

	"github.com/codegangsta/cli"
)

func MkdirIfNoExists(dir string) error {
	dir = os.ExpandEnv(dir)
	if _, err := os.Stat(dir); err != nil {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func chttp(method string, url string, v ...interface{}) (res *JSONResponse, err error) {
	var resp *http.Response
	switch method {
	case "GET":
		resp, err = http.Get(url)
	case "POST":
		resp, err = http.PostForm(url, nil)
	}
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	res = &JSONResponse{}
	err = json.NewDecoder(resp.Body).Decode(res)
	return
}

func wrapAction(f func(*cli.Context)) func(*cli.Context) {
	return func(c *cli.Context) {
		// check if serer alive
		//host := c.GlobalString("host")
		//port := c.GlobalInt("port")
		//ServeAddr(host, port)
		go exec.Command(os.Args[0], "serv").Run()
		time.Sleep(time.Millisecond * 500)
		f(c)
	}
}

func ServAction(ctx *cli.Context) {
	host := ctx.GlobalString("host")
	port := ctx.GlobalInt("port")
	ServeAddr(host, port)
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

func ShutdownAction(ctx *cli.Context) {
	res, err := chttp("POST", fmt.Sprintf("http://%s:%d/api/shutdown",
		ctx.GlobalString("host"), ctx.GlobalInt("port")))
	if err != nil {
		panic(err)
	}
	fmt.Println(res.Message)
}

func VersionAction(ctx *cli.Context) {
	fmt.Printf("Client: %s\n", GOSUV_VERSION)
	res, err := chttp("GET", fmt.Sprintf("http://%s:%d/api/version",
		ctx.GlobalString("host"), ctx.GlobalInt("port")))
	if err != nil {
		panic(err)
	}
	fmt.Printf("Server: %s\n", res.Message)
}

var app *cli.App

func init() {
	app = cli.NewApp()
	app.Version = GOSUV_VERSION
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
			Name:   "version",
			Usage:  "Show version",
			Action: wrapAction(VersionAction),
		},
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
			Usage:  "Stop running program",
			Action: StopAction,
		},
		{
			Name:   "shutdown",
			Usage:  "Shutdown server",
			Action: ShutdownAction,
		},
		{
			Name:   "serv",
			Usage:  "This command should only be called by gosuv itself",
			Action: ServAction,
		},
	}
}

const (
	GOSUV_HOME    = "$HOME/.gosuv"
	GOSUV_CONFIG  = "gosuv.json"
	GOSUV_VERSION = "0.0.1"
)

func main() {
	MkdirIfNoExists(GOSUV_HOME)
	app.RunAndExitOnError()
}
