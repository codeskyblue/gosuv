package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/imroc/req"
	"github.com/qiniu/log"
	"github.com/urfave/cli"
)

const appID = "app_8Gji4eEAdDx"

var (
	version string = "master"
	cfg     Configuration
)

type TagInfo struct {
	Version   string `json:"tag_name"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

func githubLatestVersion(repo, name string) (tag TagInfo, err error) {
	githubURL := fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", repo, name)
	r := req.New()
	h := req.Header{}
	ghToken := os.Getenv("GITHUB_TOKEN")
	if ghToken != "" {
		h["Authorization"] = "token " + ghToken
	}
	res, err := r.Get(githubURL, h)
	if err != nil {
		return
	}
	err = res.ToJSON(&tag)
	return
}

func githubUpdate(skipConfirm bool) error {
	repo, name := "soopsio", "gosuv"
	tag, err := githubLatestVersion(repo, name)
	if err != nil {
		fmt.Println("Update failed:", err)
		return err
	}
	if tag.Version == version {
		fmt.Println("No update available, already at the latest version!")
		return nil
	}

	fmt.Println("New version available -- ", tag.Version)
	fmt.Print(tag.Body)

	if !skipConfirm {
		if !askForConfirmation("Would you like to update [Y/n]? ", true) {
			return nil
		}
	}
	fmt.Printf("New version available: %s downloading ... \n", tag.Version)
	// // fetch the update and apply it
	// err = resp.Apply()
	// if err != nil {
	// 	return err
	// }
	cleanVersion := tag.Version
	if strings.HasPrefix(cleanVersion, "v") {
		cleanVersion = cleanVersion[1:]
	}
	osArch := runtime.GOOS + "_" + runtime.GOARCH

	downloadURL := StringFormat("https://github.com/{repo}/{name}/releases/download/{tag}/{name}_{version}_{os_arch}.tar.gz", map[string]interface{}{
		"repo":    "codeskyblue",
		"name":    "gosuv",
		"tag":     tag.Version,
		"version": cleanVersion,
		"os_arch": osArch,
	})
	fmt.Println("Not finished yet. download from:", downloadURL)
	// fmt.Printf("Updated to new version: %s!\n", tag.Version)
	return nil
}

func checkServerStatus() error {
	resp, err := http.Get(cfg.Client.ServerURL + "/api/status")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	var ret JSONResponse
	err = json.Unmarshal(body, &ret)
	if err != nil {
		return errors.New("json loads error: " + string(body))
	}
	if ret.Status != 0 {
		return fmt.Errorf("%v", ret.Value)
	}
	return nil
}

func main() {
	var defaultConfigPath = filepath.Join(defaultGosuvDir, "conf/config.yml")

	app := cli.NewApp()
	app.Name = "gosuv"
	app.Version = version
	app.Usage = "golang port of python-supervisor"
	app.Before = func(c *cli.Context) error {
		var err error
		cfgPath := c.GlobalString("conf")
		cfg, err = readConf(cfgPath)
		if err != nil {
			log.Fatal(err)
		}
		return nil
	}
	app.Authors = []cli.Author{
		cli.Author{
			Name:  "codeskyblue",
			Email: "codeskyblue@gmail.com",
		},
	}
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "conf, c",
			Usage: "config file",
			Value: defaultConfigPath,
		},
	}
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
					Name:  "conf, c",
					Usage: "config file",
					Value: defaultConfigPath,
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
			Name:   "start",
			Usage:  "Start program",
			Action: actionStart,
		},
		{
			Name:   "stop",
			Usage:  "Stop program",
			Action: actionStop,
		},
		{
			Name:   "reload",
			Usage:  "Reload config file",
			Action: actionReload,
		},
		{
			Name:  "shutdown",
			Usage: "Shutdown server",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "restart, r",
					Usage: "restart server(todo)",
				},
			},
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
				cli.BoolFlag{
					Name:  "yes, y",
					Usage: "Do not promote to confirm",
				},
			},
			Action: actionUpdateSelf,
		},
		{
			Name:   "edit",
			Usage:  "Edit config file",
			Action: actionEdit,
		},
		{
			Name:    "version",
			Usage:   "Show version",
			Aliases: []string{"v"},
			Action:  actionVersion,
		},
	}
	if err := app.Run(os.Args); err != nil {
		os.Exit(1)
	}
}
