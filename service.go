package main

import (
	"net"
	"os"
	"time"

	pb "github.com/codeskyblue/gosuv/gosuvpb"
	"github.com/golang/protobuf/proto"
	"golang.org/x/net/context"
)

type PbProgram struct {
}

func (this *PbProgram) Start(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	program, err := programTable.Get(in.GetName())
	if err != nil {
		return
	}
	program.InputData(EVENT_START)
	res.Message = proto.String("program started")
	return res, nil
}

func (this *PbProgram) Stop(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	program, err := programTable.Get(in.GetName())
	if err != nil {
		return
	}
	program.InputData(EVENT_STOP)
	res.Message = proto.String("program stopped")
	return res, nil
}

type PbSuvServer struct {
	lis net.Listener
}

func (s *PbSuvServer) Control(ctx context.Context, in *pb.CtrlRequest) (*pb.CtrlResponse, error) {
	res := &pb.CtrlResponse{}
	res.Value = proto.String("Hi")
	return res, nil
}

func (s *PbSuvServer) Shutdown(ctx context.Context, in *pb.NopRequest) (*pb.Response, error) {
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.lis.Close()
		os.Exit(2)
	}()
	res := &pb.Response{}
	res.Code = proto.Int32(200)
	return res, nil
}

func (s *PbSuvServer) Version(ctx context.Context, in *pb.NopRequest) (res *pb.Response, err error) {
	res = &pb.Response{
		Message: proto.String(GOSUV_VERSION),
	}
	return
}
