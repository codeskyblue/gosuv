package main

import (
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"

	pb "github.com/codeskyblue/gosuv/gosuvpb"
	"github.com/qiniu/log"
	"google.golang.org/grpc"
)

func handleSignal(lis net.Listener) {
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		for sig := range sigc {
			log.Println("Receive signal:", sig)
			if sig == syscall.SIGHUP {
				return // ignore, when shell session closed, gosuv will receive SIGHUP signal
			}
			lis.Close()
			log.Println("Kill all running gosuv programs")
			programTable.StopAll()
			os.Exit(0)
			return
		}
	}()
}

type PbSuvServer struct {
	lis net.Listener
}

func RunGosuvService(addr string) error {
	initProgramTable()

	lis, err := net.Listen("unix", GOSUV_SOCK_PATH)
	if err != nil {
		log.Fatal(err)
	}
	handleSignal(lis)

	pbServ := &PbSuvServer{}
	pbProgram := &PbProgram{}

	grpcServ := grpc.NewServer()
	pb.RegisterGoSuvServer(grpcServ, pbServ)
	pb.RegisterProgramServer(grpcServ, pbProgram)

	pbServ.lis = lis
	grpcServ.Serve(lis)
	return fmt.Errorf("Address: %s has been used", addr)
}
