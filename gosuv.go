package main

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/codegangsta/inject"
	. "github.com/codeskyblue/gosuv/config"
	pb "github.com/codeskyblue/gosuv/gosuvpb"
	. "github.com/codeskyblue/gosuv/utils"
	"github.com/qiniu/log"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

var (
	GitCommit  string
	GitBranch  string
	GitState   string
	GitSummary string
	BuildDate  string

	GOSUV_VERSION = "UNKNOWN"
)

func MkdirIfNoExists(dir string) error {
	dir = os.ExpandEnv(dir)
	if _, err := os.Stat(dir); err != nil {
		return os.MkdirAll(dir, 0755)
	}
	return nil
}

func connect(ctx *cli.Context) (cc *grpc.ClientConn, err error) {
	conn, err := grpcDial("unix", GOSUV_SOCK_PATH)
	return conn, err
}

func startBackgroundServer() error {
	cmd := exec.Command(os.Args[0], "serv")
	timeout := time.Millisecond * 500
	er := <-GoTimeoutFunc(timeout, cmd.Run)
	if er == ErrGoTimeout {
		return nil
	}
	return fmt.Errorf("server stared failed, %v", er)
}

func dialWithRetry(network, address string) (conn *grpc.ClientConn, err error) {
	if network == "unix" {
		if _, er := os.Stat(address); er != nil {
			if err = startBackgroundServer(); err != nil {
				log.Warnf("Server started failed.")
				return nil, err
			} else {
				log.Info("gosuv server started")
			}
		}
	}
	return grpcDial(network, address)
}

func wrap(f interface{}) func(*cli.Context) {
	return func(ctx *cli.Context) {
		if ctx.GlobalBool("debug") {
			log.SetOutputLevel(log.Ldebug)
		}

		conn, err := dialWithRetry("unix", GOSUV_SOCK_PATH)
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
	RunGosuvService(addr)
}

func ActionStatus(client pb.GoSuvClient) {
	res, err := client.Status(context.Background(), &pb.NopRequest{})
	if err != nil {
		log.Fatal(err)
	}
	for _, ps := range res.GetPrograms() {
		fmt.Printf("%-10s\t%-8s\t%s\n", ps.Name, ps.Status, ps.Extra)
	}
}

func ActionAdd(ctx *cli.Context, client pb.GoSuvClient) {
	name := ctx.String("name")
	if name == "" {
		name = filepath.Base(ctx.Args()[0])
	}

	dir, _ := os.Getwd()

	if len(ctx.Args()) < 1 {
		log.Fatal("need at least one args")
	}
	cmdName := ctx.Args().First()
	cmdPath, err := exec.LookPath(cmdName)
	if err != nil {
		log.Fatal(err)
	}

	req := new(pb.ProgramInfo)
	req.Name = ctx.String("name")
	req.Directory = dir
	req.Command = append([]string{cmdPath}, ctx.Args().Tail()...)
	req.Environ = ctx.StringSlice("env")

	res, err := client.Create(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Message)
}

// func buildURI(ctx *cli.Context, uri string) string {
// 	return fmt.Sprintf("http://%s%s", ctx.GlobalString("addr"), uri)
// }

func ActionStop(ctx *cli.Context) {
	conn, err := connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	name := ctx.Args().First()
	client := pb.NewProgramClient(conn)
	res, err := client.Stop(context.Background(), &pb.Request{Name: name})
	if err != nil {
		Errorf("ERR: %#v\n", err)
	}
	fmt.Println(res.Message)
}

func ActionRemove(ctx *cli.Context, client pb.ProgramClient) {
	name := ctx.Args().First()
	reader := bufio.NewReader(os.Stdin)
	fmt.Print("Danger operation, Enter name again: ")
	text, _ := reader.ReadString('\n')
	text = strings.TrimSpace(text)
	if text != name {
		fmt.Println("Canceled.")
		return
	}

	res, err := client.Remove(context.Background(), &pb.Request{Name: name})
	if err != nil {
		Errorf("ERR: %#v\n", err)
	}
	fmt.Println(res.Message)
}

func ActionTail(ctx *cli.Context, client pb.ProgramClient) {
	req := &pb.TailRequest{
		Name:   ctx.Args().First(),
		Number: int32(ctx.Int("number")),
		Follow: ctx.Bool("follow"),
	}
	tailc, err := client.Tail(context.Background(), req)
	if err != nil {
		log.Fatal(err)
	}
	for {
		line, err := tailc.Recv()
		if err == io.EOF {
			return
		}
		if err != nil {
			log.Fatal(err)
		}
		fmt.Print(line.Line)
	}
}

func Errorf(format string, v ...interface{}) {
	fmt.Printf(format, v...)
	os.Exit(1)
}

func ActionStart(ctx *cli.Context) {
	conn, err := connect(ctx)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	name := ctx.Args().First()
	client := pb.NewProgramClient(conn)
	res, err := client.Start(context.Background(), &pb.Request{Name: name})
	if err != nil {
		Errorf("ERR: %#v\n", err)
	}
	fmt.Println(res.Message)
}

// grpc.Dial can't set network, so I have to implement this func
func grpcDial(network, addr string) (*grpc.ClientConn, error) {
	return grpc.Dial(addr,
		grpc.WithInsecure(),
		grpc.WithTimeout(time.Second*2),
		grpc.WithDialer(
			func(address string, timeout time.Duration) (conn net.Conn, err error) {
				return net.DialTimeout(network, address, timeout)
			}))
}

func ActionShutdown(ctx *cli.Context) {
	conn, err := connect(ctx)
	if err != nil {
		fmt.Println("server already closed")
		return
	}
	defer conn.Close()

	client := pb.NewGoSuvClient(conn)
	res, err := client.Shutdown(context.Background(), &pb.NopRequest{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(res.Message)
}

func ActionVersion(ctx *cli.Context, client pb.GoSuvClient) {
	fmt.Printf("gosuv version:\n  client: %s\n", GOSUV_VERSION)
	res, err := client.Version(context.Background(), &pb.NopRequest{})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("  server: %s\n", res.Message)
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
			Action: wrap(ActionVersion),
		},
		{
			Name:    "status",
			Aliases: []string{"st"},
			Usage:   "show program status",
			Action:  wrap(ActionStatus),
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
			Action: wrap(ActionAdd),
		},
		{
			Name:   "start",
			Usage:  "start a not running program",
			Action: wrap(ActionStart),
		},
		{
			Name:   "stop",
			Usage:  "Stop running program",
			Action: wrap(ActionStop),
		},
		{
			Name:   "remove",
			Usage:  "Remove program",
			Action: wrap(ActionRemove),
		},
		{
			Name:   "tail",
			Usage:  "tail log",
			Action: wrap(ActionTail),
			Flags: []cli.Flag{
				cli.IntFlag{
					Name:  "number, n",
					Value: 10,
					Usage: "The location is number lines.",
				},
				cli.BoolTFlag{
					Name:  "follow, f",
					Usage: "Constantly show log",
				},
			},
		},
		{
			Name:   "shutdown",
			Usage:  "Shutdown server",
			Action: ActionShutdown,
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
	selfPath, _ := filepath.Abs(os.Args[0])
	envs := []string{
		"GOSUV_SERVER_ADDR=" + ctx.GlobalString("addr"),
		"GOSUV_PLUGIN_NAME=" + name,
		"GOSUV_PROGRAM=" + selfPath,
	}
	cmd := exec.Command(filepath.Join(pluginDir, "run"), ctx.Args()...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = os.Stdin
	cmd.Dir = pluginDir
	cmd.Env = append(os.Environ(), envs...)
	cmd.Run()
}

func main() {
	MkdirIfNoExists(GOSUV_HOME)

	loadRConfig()
	initCli()

	app.HideHelp = false
	app.HideVersion = true
	app.Run(os.Args)
}
