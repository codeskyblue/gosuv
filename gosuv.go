package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/codegangsta/cli"
	"github.com/franela/goreq"
	"github.com/qiniu/log"
)

var (
	CMDPLUGIN_DIR = filepath.Join(GOSUV_HOME, "cmdplugin")
)

func MkdirIfNoExists(dir string) error {
	dir = os.ExpandEnv(dir)
	if _, err := os.Stat(dir); err != nil {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func ShellTestFile(flag string, file string) bool {
	finfo, err := os.Stat(file)
	switch flag {
	case "x":
		if err == nil && (finfo.Mode()&os.ModeExclusive) == 0 {
			return true
		}
	}
	return false
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
		_, err := goreq.Request{
			Method: "GET",
			Uri:    buildURI(c, "/api/version"),
		}.Do()
		if err != nil {
			go exec.Command(os.Args[0], "serv").Run()
			time.Sleep(time.Millisecond * 500)
		}
		f(c)
	}
}

func ServAction(ctx *cli.Context) {
	host := ctx.GlobalString("host")
	port := ctx.GlobalInt("port")
	ServeAddr(host, port)
}

func StatusAction(ctx *cli.Context) {
	programs := make([]*Program, 0)
	res, err := goreq.Request{
		Method: "GET",
		Uri:    buildURI(ctx, "/api/programs"),
	}.Do()
	if err != nil {
		log.Fatal(err)
	}
	if res.StatusCode != http.StatusOK {
		log.Fatal(res.Body.ToString())
	}
	if err = res.Body.FromJsonTo(&programs); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("%10s\t%s\n", "NAME", "STATUS")
	for _, p := range programs {
		fmt.Printf("%10s\t%s\n", p.Info.Name, p.Status)
	}
}

func AddAction(ctx *cli.Context) {
	name := ctx.String("name")
	dir, _ := os.Getwd()
	if len(ctx.Args()) < 1 {
		log.Fatal("need at least one args")
	}
	if name == "" {
		name = ctx.Args()[0]
	}
	log.Println(ctx.Args().Tail())
	log.Println([]string(ctx.Args()))
	log.Println(ctx.Args().Tail())
	log.Println(ctx.StringSlice("env"))
	log.Println("Dir:", dir)
	cmdName := ctx.Args().First()
	log.Println("cmd name:", cmdName)
	cmdPath, err := exec.LookPath(cmdName)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("program: %s has been added\n", strconv.Quote(name))
	p := &ProgramInfo{
		Name:    name,
		Dir:     dir,
		Command: append([]string{cmdPath}, ctx.Args().Tail()...),
		Environ: ctx.StringSlice("env"),
	}
	res, err := goreq.Request{
		Method: "POST",
		Uri:    buildURI(ctx, "/api/programs"),
		Body:   p,
	}.Do()
	if err != nil {
		log.Fatal(err)
	}
	var jres JSONResponse
	if res.StatusCode != http.StatusOK {
		log.Fatal(res.Body.ToString())
	}
	if err = res.Body.FromJsonTo(&jres); err != nil {
		log.Fatal(err)
	}
	fmt.Println(jres.Message)
}

func buildURI(ctx *cli.Context, uri string) string {
	return fmt.Sprintf("http://%s:%d%s",
		ctx.GlobalString("host"), ctx.GlobalInt("port"), uri)
}

func buildpbURI(ctx *cli.Context) string {
	return buildURI(ctx, "/protobuf")
}

func StopAction(ctx *cli.Context) {
	log.Println(buildpbURI(ctx))
}

func ShutdownAction(ctx *cli.Context) {
	res, err := chttp("POST", fmt.Sprintf("http://%s:%d/api/shutdown",
		ctx.GlobalString("host"), ctx.GlobalInt("port")))
	if err != nil {
		log.Println("Already shutdown")
		return
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
			Name:    "status",
			Aliases: []string{"st"},
			Usage:   "show program status",
			Action:  StatusAction,
		},
		{
			Name:  "add",
			Usage: "add to running list",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "name, n",
					Usage: "program name",
				},
				cli.StringSliceFlag{
					Name:  "env, e",
					Usage: "Specify environ",
				},
			},
			Action: wrapAction(AddAction),
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
	finfos, err := ioutil.ReadDir(CMDPLUGIN_DIR)
	if err != nil {
		return
	}
	for _, finfo := range finfos {
		if !finfo.IsDir() {
			continue
		}
		//modeExec := os.FileMode(0500)
		//if strings.HasPrefix(finfo.Name(), "gosuv-") && (finfo.Mode()&modeExec) == modeExec {
		//cmdName := string(finfo.Name()[6:])
		cmdName := finfo.Name()
		app.Commands = append(app.Commands, cli.Command{
			Name:   cmdName,
			Usage:  "Plugin command",
			Action: newPluginAction(cmdName),
		})
	}
}

func newPluginAction(name string) func(*cli.Context) {
	return func(ctx *cli.Context) {
		runPlugin(ctx, name)
	}
}

func runPlugin(ctx *cli.Context, name string) {
	serverAddr := fmt.Sprintf("%s:%d",
		ctx.GlobalString("host"), ctx.GlobalInt("port"))
	pluginDir := filepath.Join(CMDPLUGIN_DIR, name)
	envs := []string{
		"GOSUV_SERVER_ADDR=" + serverAddr,
		"GOSUV_PLUGIN_NAME=" + name,
	}
	cmd := exec.Command(filepath.Join(pluginDir, "run"), ctx.Args()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = pluginDir
	cmd.Env = append(os.Environ(), envs...)
	cmd.Run()
}

var (
	GOSUV_HOME           = os.ExpandEnv("$HOME/.gosuv")
	GOSUV_CONFIG         = filepath.Join(GOSUV_HOME, "gosuv.json")
	GOSUV_PROGRAM_CONFIG = filepath.Join(GOSUV_HOME, "programs.json")
	GOSUV_VERSION        = "0.0.1"
)

func main() {
	MkdirIfNoExists(GOSUV_HOME)
	app.HideHelp = false
	app.RunAndExitOnError()
}
