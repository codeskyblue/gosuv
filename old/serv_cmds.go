package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"time"

	pb "github.com/codeskyblue/gosuv/gosuvpb"
	. "github.com/codeskyblue/gosuv/program"
	"golang.org/x/net/context"
)

func (s *PbSuvServer) Create(ctx context.Context, in *pb.ProgramInfo) (*pb.Response, error) {
	pinfo := new(ProgramInfo)
	pinfo.Name = in.Name
	pinfo.Command = in.Command
	pinfo.Dir = in.Directory
	pinfo.Environ = in.Environ
	pinfo.AutoStart = true

	log.Println(in.Name)
	program := NewProgram(pinfo)
	if err := programTable.AddProgram(program); err != nil {
		return nil, err
	}
	program.InputData(EVENT_START)
	res := new(pb.Response)
	res.Code = 200
	res.Message = fmt.Sprintf("%s: created", pinfo.Name)
	return res, nil
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

type PbProgram struct{}

func (this *PbProgram) Start(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	program, err := programTable.Get(in.Name)
	if err != nil {
		return
	}
	program.InputData(EVENT_START)
	if program.Status == ST_RUNNING {
		res.Message = in.Name + ": already started"
	} else {
		res.Message = in.Name + ": started"
	}
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

// Remove(ctx context.Context, in *Request, opts ...grpc.CallOption) (*Response, error)
func (s *PbProgram) Remove(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	err = programTable.Remove(in.Name)
	res.Message = fmt.Sprintf("Program %s removed.", in.Name)
	return
}

//func (this *PbProgram) Tail(ctx context.Context, in *pb.Request)(stream
func (c *PbProgram) Tail(in *pb.TailRequest, stream pb.Program_TailServer) (err error) {
	program, err := programTable.Get(in.Name)
	if err != nil {
		return
	}
	args := []string{"-n", fmt.Sprintf("%d", in.Number)}
	if in.Follow {
		args = append(args, "-f")
	}
	cmd := exec.Command("tail", append(args, program.LogFilePath())...)
	rd, err := cmd.StdoutPipe()
	go cmd.Run()
	defer func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	}()
	buf := make([]byte, 1024)
	for {
		nr, err := rd.Read(buf)
		if err != nil {
			break
		}
		line := &pb.LogLine{Line: string(buf[:nr])}
		if err = stream.Send(line); err != nil {
			return err
		}
	}
	return nil
}
