package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	pb "github.com/codeskyblue/gosuv/gosuvpb"
	"github.com/qiniu/log"
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

/*
func versionHandler(w http.ResponseWriter, r *http.Request) {
	renderJSON(w, &JSONResponse{
		Code:    200,
		Message: GOSUV_VERSION,
	})
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

	program := NewProgram(pinfo)
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
*/

func handleSignal(lis net.Listener) {
	sigc := make(chan os.Signal, 2)
	signal.Notify(sigc, syscall.SIGINT, syscall.SIGHUP)
	go func() {
		for sig := range sigc {
			log.Println("Receive signal:", sig)
			if sig == syscall.SIGHUP {
				return // ignore
			}
			lis.Close()
			programTable.StopAll()
			os.Exit(0)
			return
		}
	}()
}

func ServeAddr(addr string) error {
	InitServer()

	/*
		t := tango.New()
		t.Group("/api", func(g *tango.Group) {
			g.Get("/version", versionHandler)
			g.Post("/shutdown", shutdownHandler)
			g.Post("/programs", addHandler)
			g.Get("/programs", statusHandler)
		})

		go t.Run(addr)
	*/

	lis, err := net.Listen("unix", filepath.Join(GOSUV_HOME, "gosuv.sock"))
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
