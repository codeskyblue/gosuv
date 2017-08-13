package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/franela/goreq"
	"github.com/goji/httpauth"
	"github.com/urfave/cli"
)

func postForm(pathname string, data url.Values) (r JSONResponse, err error) {
	resp, err := http.PostForm(cfg.Client.ServerURL+pathname, data)
	if err != nil {
		return r, err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return r, err
	}
	err = json.Unmarshal(body, &r)
	if err != nil {
		return r, fmt.Errorf("POST %v %v", strconv.Quote(pathname), string(body))
	}
	return r, nil
}

func actionStartServer(c *cli.Context) error {
	suv, hdlr, err := newSupervisorHandler()
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
		suv.AutoStartPrograms()
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

func actionStatus(c *cli.Context) error {
	res, err := goreq.Request{
		Uri: cfg.Client.ServerURL + "/api/programs",
	}.Do()
	if err != nil {
		return err
	}
	var programs = make([]struct {
		Program Program `json:"program"`
		Status  string  `json:"status"`
	}, 0)
	if err := res.Body.FromJsonTo(&programs); err != nil {
		return err
	}
	format := "%-23s\t%-8s\n"
	fmt.Printf(format, "PROGRAM NAME", "STATUS")
	for _, p := range programs {
		fmt.Printf(format, p.Program.Name, p.Status)
	}
	return nil
}

// cmd: <start|stop>
func programOperate(cmd, name string) (err error, success bool) {
	res, err := goreq.Request{
		Method: "POST",
		Uri:    cfg.Client.ServerURL + "/api/programs/" + name + "/" + cmd,
	}.Do()
	if err != nil {
		return
	}
	var v = struct {
		Status int `json:"status"`
	}{}
	if err = res.Body.FromJsonTo(&v); err != nil {
		return
	}
	success = v.Status == 0
	return
}

func actionStart(c *cli.Context) (err error) {
	name := c.Args().First()
	err, success := programOperate("start", name)
	if err != nil {
		return
	}
	if success {
		fmt.Println("Started")
	} else {
		fmt.Println("Start failed")
	}
	return nil
}

func actionStop(c *cli.Context) (err error) {
	name := c.Args().First()
	err, success := programOperate("stop", name)
	if err != nil {
		return
	}
	if !success {
		fmt.Println("Stop failed")
	}
	return nil
}

func actionShutdown(c *cli.Context) error {
	restart := c.Bool("restart")
	if restart {
		log.Fatal("Restart not implemented.")
	}
	ret, err := postForm("/api/shutdown", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ret.Value)
	return nil
}

func actionUpdateSelf(c *cli.Context) error {
	return githubUpdate(c.Bool("yes"))
}

func actionEdit(c *cli.Context) error {
	cmd := exec.Command("vim", filepath.Join(os.Getenv("HOME"), ".gosuv/programs.yml"))
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func actionVersion(c *cli.Context) error {
	fmt.Printf("gosuv version %s\n", version)
	return nil
}

func actionReload(c *cli.Context) error {
	ret, err := postForm("/api/reload", nil)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(ret.Value)
	return nil
}

func actionConfigTest(c *cli.Context) error {
	if _, _, err := newSupervisorHandler(); err != nil {
		log.Fatal(err)
	}
	log.Println("test is successful")
	return nil
}
