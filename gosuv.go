package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"time"

	"github.com/codegangsta/cli"
	"github.com/codegangsta/inject"
	pb "github.com/codeskyblue/gosuv/gosuvpb"
	"github.com/franela/goreq"
	"github.com/golang/protobuf/proto"
	"github.com/qiniu/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
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

func connect(ctx *cli.Context) (cc *grpc.ClientConn, err error) {
	sockPath := filepath.Join(GOSUV_HOME, "gosuv.sock")
	conn, err := grpcDial("unix", sockPath)
	return conn, err
}

func testConnection(network, address string) error {
	log.Debugf("test connection")
	testconn, err := net.DialTimeout(network, address, time.Millisecond*100)
	if err != nil {
		log.Debugf("start run server")
		cmd := exec.Command(os.Args[0], "serv")
		timeout := time.Millisecond * 500
		er := <-GoTimeoutFunc(timeout, cmd.Run)
		if er == ErrGoTimeout {
			fmt.Println("server started")
		} else {
			return fmt.Errorf("server stared failed, %v", er)
		}
	} else {
		testconn.Close()
	}
	return nil
}

func wrap(f interface{}) func(*cli.Context) {
	return func(ctx *cli.Context) {
		if ctx.GlobalBool("debug") {
			log.SetOutputLevel(log.Ldebug)
		}

		sockPath := filepath.Join(GOSUV_HOME, "gosuv.sock")
		if err := testConnection("unix", sockPath); err != nil {
			log.Fatal(err)
		}

		conn, err := connect(ctx)
		if err != nil {
			log.Fatal(err)
		}
		defer conn.Close()
		programClient := pb.NewProgramClient(conn)
		gosuvClient := pb.NewGoSuvClient(conn)

		inj := inject.New()
		inj.Map(programClient)
		inj.Map(gosuvClient)
		inj.Map(ctx)
		inj.Invoke(f)
	}
}

func ServAction(ctx *cli.Context) {
	addr := ctx.GlobalString("addr")
	ServeAddr(addr)
}

func StatusAction(client pb.GoSuvClient) {
	res, err := client.Status(context.Background(), &pb.NopRequest{})
	if err != nil {
		log.Fatal(err)
	}
	for _, ps := range res.GetPrograms() {
		fmt.Printf("%-10s\t%-8s\t%s\n", ps.GetName(), ps.GetStatus(), ps.GetExtra())
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
	return fmt.Sprintf("http://%s%s", ctx.GlobalString("addr"), uri)
}

func StopAction(ctx *cli.Context) {
	conn, err := connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	name := ctx.Args().First()
	client := pb.NewProgramClient(conn)
	res, err := client.Stop(context.Background(), &pb.Request{Name: proto.String(name)})
	if err != nil {
		Errorf("ERR: %#v\n", err)
	}
	fmt.Println(res.GetMessage())
}

func Errorf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
	os.Exit(1)
}

func StartAction(ctx *cli.Context) {
	conn, err := connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	name := ctx.Args().First()
	client := pb.NewProgramClient(conn)
	res, err := client.Start(context.Background(), &pb.Request{Name: proto.String(name)})
	if err != nil {
		Errorf("ERR: %#v\n", err)
	}
	fmt.Println(res.GetMessage())
}

// grpc.Dial can't set network, so I have to implement this func
func grpcDial(network, addr string) (*grpc.ClientConn, error) {
	return grpc.Dial(addr, grpc.WithInsecure(), grpc.WithDialer(
		func(address string, timeout time.Duration) (conn net.Conn, err error) {
			return net.DialTimeout(network, address, timeout)
		}))
}

func ShutdownAction(ctx *cli.Context, client pb.GoSuvClient) {
	res, err := client.Shutdown(context.Background(), &pb.NopRequest{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.GetMessage())
}

func VersionAction(ctx *cli.Context, client pb.GoSuvClient) {
	fmt.Printf("Client: %s\n", GOSUV_VERSION)
	res, err := client.Version(context.Background(), &pb.NopRequest{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Server: %s\n", res.GetMessage())
}

var app *cli.App

func initCli() {
	app = cli.NewApp()
	app.Version = GOSUV_VERSION
	app.Name = "gosuv"
	app.Usage = "supervisor your program"
	app.HideHelp = true
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:   "addr",
			Value:  rcfg.Server.RpcAddr,
			Usage:  "server address",
			EnvVar: "GOSUV_SERVER_ADDR",
		},
		cli.BoolFlag{
			Name:   "debug, d",
			Usage:  "enable debug info",
			EnvVar: "GOSUV_DEBUG",
		},
	}

	app.Commands = []cli.Command{
		{
			Name:   "version",
			Usage:  "Show version",
			Action: wrap(VersionAction),
		},
		{
			Name:    "status",
			Aliases: []string{"st"},
			Usage:   "show program status",
			Action:  wrap(StatusAction),
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
			Action: wrap(AddAction),
		},
		{
			Name:   "start",
			Usage:  "start a not running program",
			Action: wrap(StartAction),
		},
		{
			Name:   "stop",
			Usage:  "Stop running program",
			Action: wrap(StopAction),
		},
		{
			Name:   "shutdown",
			Usage:  "Shutdown server",
			Action: wrap(ShutdownAction),
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
	pluginDir := filepath.Join(CMDPLUGIN_DIR, name)
	envs := []string{
		"GOSUV_SERVER_ADDR=" + ctx.GlobalString("addr"),
		"GOSUV_PLUGIN_NAME=" + name,
		"GOSUV_FILE_PATH=" + os.Args[0],
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

	if err := loadRConfig(); err != nil {
		log.Fatal(err)
	}
	initCli()

	app.HideHelp = false
	app.HideVersion = true
	app.RunAndExitOnError()
}
