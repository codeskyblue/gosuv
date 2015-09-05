package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	pb "github.com/codeskyblue/gosuv/gosuvpb"
	"github.com/golang/protobuf/proto"
	"github.com/lunny/log"
	"github.com/lunny/tango"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
)

type JSONResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func renderJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Add("Content-Type", "json")
	json.NewEncoder(w).Encode(v)
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	renderJSON(w, &JSONResponse{
		Code:    200,
		Message: GOSUV_VERSION,
	})
}

func buildProgram(pinfo *ProgramInfo) (*Program, error) {
	// init cmd
	cmd := exec.Command(pinfo.Command[0], pinfo.Command[1:]...)
	cmd.Dir = pinfo.Dir
	cmd.Env = append(os.Environ(), pinfo.Environ...)
	program := NewProgram(cmd, pinfo)

	// set output
	return program, nil
}

func statusHandler(w http.ResponseWriter, r *http.Request) {
	prms := programTable.Programs()
	renderJSON(w, prms)
}

func addHandler(w http.ResponseWriter, r *http.Request) {
	pinfo := new(ProgramInfo)
	err := json.NewDecoder(r.Body).Decode(pinfo)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	log.Printf("add: %#v", pinfo)

	program, err := buildProgram(pinfo)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	if err = programTable.AddProgram(program); err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	program.InputData(EVENT_START)

	renderJSON(w, &JSONResponse{
		Code:    200,
		Message: "program add success",
	})
}

func shutdownHandler(w http.ResponseWriter, r *http.Request) {
	go func() {
		time.Sleep(50 * time.Millisecond)
		os.Exit(2)
	}()
	renderJSON(w, &JSONResponse{
		Code:    200,
		Message: "not implement",
	})
}

type SuvServer struct {
	lis net.Listener
}

func (s *SuvServer) Control(ctx context.Context, in *pb.CtrlRequest) (*pb.CtrlResponse, error) {
	res := &pb.CtrlResponse{}
	res.Value = proto.String("Hi")
	return res, nil
}

func (s *SuvServer) Shutdown(ctx context.Context, in *pb.NopRequest) (*pb.Response, error) {
	go func() {
		time.Sleep(50 * time.Millisecond)
		s.lis.Close()
		os.Exit(2)
	}()
	res := &pb.Response{}
	res.Code = proto.Int32(200)
	return res, nil
}

func ServeAddr(host string, port int) error {
	InitServer()

	t := tango.New()
	t.Group("/api", func(g *tango.Group) {
		g.Get("/version", versionHandler)
		g.Post("/shutdown", shutdownHandler)
		g.Post("/programs", addHandler)
		g.Get("/programs", statusHandler)
	})

	addr := fmt.Sprintf("%s:%d", host, port)
	wg := &sync.WaitGroup{}
	wg.Add(2)
	go func() {
		t.Run(addr)
		wg.Done()
	}()
	go func() {
		grpcServ := grpc.NewServer()
		pbServ := &SuvServer{}
		pb.RegisterGoSuvServer(grpcServ, pbServ)

		lis, err := net.Listen("unix", filepath.Join(GOSUV_HOME, "gosuv.sock"))
		if err != nil {
			log.Fatal(err)
		}
		pbServ.lis = lis
		//defer lis.Close()
		grpcServ.Serve(lis)
		wg.Done()
	}()
	wg.Wait()
	return fmt.Errorf("Address: %s has been used", addr)
}
