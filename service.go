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
	res.Message = proto.String(in.GetName() + ": started")
	return res, nil
}

func (this *PbProgram) Stop(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	program, err := programTable.Get(in.GetName())
	if err != nil {
		return
	}
	program.InputData(EVENT_STOP)
	res.Message = proto.String(in.GetName() + ": stopped")
	return res, nil
}

type PbSuvServer struct {
	lis net.Listener
}

func (s *PbSuvServer) Shutdown(ctx context.Context, in *pb.NopRequest) (*pb.Response, error) {
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.lis.Close()
		programTable.StopAll()
		os.Exit(2)
	}()
	res := &pb.Response{}
	res.Message = proto.String("gosuv shutdown")
	return res, nil
}

func (s *PbSuvServer) Version(ctx context.Context, in *pb.NopRequest) (res *pb.Response, err error) {
	res = &pb.Response{
		Message: proto.String(GOSUV_VERSION),
	}
	return
}

func (s *PbSuvServer) Status(ctx context.Context, in *pb.NopRequest) (res *pb.StatusResponse, err error) {
	res = &pb.StatusResponse{}
	for _, program := range programTable.Programs() {
		ps := &pb.ProgramStatus{}
		ps.Name = proto.String(program.Info.Name)
		ps.Status = proto.String(program.Status)
		ps.Extra = proto.String("...")
		res.Programs = append(res.Programs, ps)
	}
	return
}
