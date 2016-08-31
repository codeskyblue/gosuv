package main

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/go-yaml/yaml"
)

type Configuration struct {
	Server struct {
		HttpAuth struct {
			User     string `yaml:"username"`
			Password string `yaml:"password"`
		} `yaml:"httpauth"`
		Addr string `yaml:"addr"`
	} `yaml:"server"`

	Client struct {
		ServerURL string `yaml:"server_url"`
	}
}

func readConf(filename string) (c Configuration, err error) {
	// initial default value
	c.Server.Addr = ":11313" // in memory of 08-31 13:13
	c.Client.ServerURL = "http://localhost:11313"

	data, err := ioutil.ReadFile(filename)
	if err != nil {
		data = []byte("")
	}
	err = yaml.Unmarshal(data, &c)
	if err != nil {
		return
	}
	cfgDir := filepath.Dir(filename)
	if !IsDir(cfgDir) {
		os.MkdirAll(cfgDir, 0755)
	}
	data, _ = yaml.Marshal(c)
	err = ioutil.WriteFile(filename, data, 0644)
	return
}
