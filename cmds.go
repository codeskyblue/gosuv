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
	err := checkServerStatus()
	if err != nil {
		log.Fatal(err)
	} else {
		log.Println("Server is running, OK.")
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
	return equinoxUpdate(c.String("channel"), c.Bool("yes"))
}

func actionEdit(c *cli.Context) error {
	cmd := exec.Command("vim", filepath.Join(os.Getenv("HOME"), ".gosuv/programs.yml"))
	cmd.Stdout = os.Stdout
	cmd.Stdin = os.Stdin
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func actionVersion(c *cli.Context) error {
	fmt.Printf("gosuv version %s\n", Version)
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
