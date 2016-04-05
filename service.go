package main

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	pb "github.com/codeskyblue/gosuv/gosuvpb"
	"github.com/qiniu/log"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type PbProgram struct{}

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

// Remove(ctx context.Context, in *Request, opts ...grpc.CallOption) (*Response, error)
func (s *PbProgram) Remove(ctx context.Context, in *pb.Request) (res *pb.Response, err error) {
	res = &pb.Response{}
	res.Message = "TODO: remove not finished. " + in.Name
	return res, nil
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
	cmd := exec.Command("tail", append(args, program.logFilePath())...)
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

type PbSuvServer struct {
	lis net.Listener
}

func (s *PbSuvServer) Create(ctx context.Context, in *pb.ProgramInfo) (*pb.Response, error) {
	pinfo := new(ProgramInfo)
	pinfo.Name = in.Name
	pinfo.Command = in.Command
	pinfo.Dir = in.Directory
	pinfo.Environ = in.Environ

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
			programTable.StopAll()
			os.Exit(0)
			return
		}
	}()
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
