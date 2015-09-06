package main

import "gopkg.in/gcfg.v1"

type RCServer struct {
	WebAddr string `gcfg:"web-addr"`
}

type RConfig struct {
	Server struct {
		WebAddr string `gcfg:"web-addr"`
		RpcAddr string `gcfg:"rpc-addr"`
	}
}

var rcfg *RConfig

func loadRConfig() (err error) {
	rcfg = new(RConfig)
	// set default values
	rcfg.Server.RpcAddr = "127.0.0.1:54637"
	rcfg.Server.WebAddr = "127.0.0.1:54000"
	err = gcfg.ReadFileInto(rcfg, "gosuvrc")
	return
}
