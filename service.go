package main

import (
	"net"
	"os"
	"time"

	pb "github.com/codeskyblue/gosuv/gosuvpb"
	"golang.org/x/net/context"
)

type PbProgram struct {
}

func (this *PbProgram) Start(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	program, err := programTable.Get(in.Name)
	if err != nil {
		return
	}
	program.InputData(EVENT_START)
	res.Message = in.Name + ": started"
	return res, nil
}

func (this *PbProgram) Stop(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	program, err := programTable.Get(in.Name)
	if err != nil {
		return
	}
	program.InputData(EVENT_STOP)
	res.Message = in.Name + ": stopped"
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
	res.Message = "gosuv shutdown"
	return res, nil
}

func (s *PbSuvServer) Version(ctx context.Context, in *pb.NopRequest) (res *pb.Response, err error) {
	res = &pb.Response{
		Message: GOSUV_VERSION,
	}
	return
}

func (s *PbSuvServer) Status(ctx context.Context, in *pb.NopRequest) (res *pb.StatusResponse, err error) {
	res = &pb.StatusResponse{}
	for _, program := range programTable.Programs() {
		ps := &pb.ProgramStatus{}
		ps.Name = program.Info.Name
		ps.Status = program.Status
		ps.Extra = "..."
		res.Programs = append(res.Programs, ps)
	}
	return
}
