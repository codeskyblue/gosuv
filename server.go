package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/codeskyblue/kproc"
	"github.com/gorilla/mux"
	"github.com/qiniu/log"
)

type JSONResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type ProgramInfo struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Dir     string   `json:"dir"`
	Environ []string `json:"environ"`
}

var programTable struct {
	table map[string]*Program
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

func addHandler(w http.ResponseWriter, r *http.Request) {
	pinfo := new(ProgramInfo)
	err := json.NewDecoder(r.Body).Decode(pinfo)
	if err != nil {
		http.Error(w, err.Error(), 502)
		return
	}
	log.Printf("add: %#v", pinfo)

	// init cmd
	cmd := exec.Command(pinfo.Command[0], pinfo.Command[1:]...)
	cmd.Dir = pinfo.Dir
	cmd.Env = append(os.Environ(), pinfo.Environ...)
	program := NewProgram(cmd, pinfo)

	// set output
	logFd, err := program.createLog()
	if err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	cmd.Stdout = logFd
	cmd.Stderr = logFd

	if err = program.Start(); err != nil {
		http.Error(w, err.Error(), 503)
		return
	}
	program.Status = ST_RUNNING

	// wait func finish
	go func() {
		finish := false
		ch := GoFunc(program.Wait)
		for !finish {
			select {
			case err := <-ch:
				if err != nil {
					log.Warnf("program finish: %v", err)
				}
				finish = true
			case sig := <-program.Sig:
				program.Terminate(sig)
			}
		}
	}()
	renderJSON(w, &JSONResponse{
		Code:    200,
		Message: "program add success",
	})
}

func GoFunc(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

const (
	ST_PENDING = "pending"
	ST_RUNNING = "running"
	ST_STOPPED = "stopped"
)

type Program struct {
	*kproc.Process
	Status string `json:"state"`
	Sig    chan os.Signal
	info   *ProgramInfo
}

func NewProgram(cmd *exec.Cmd, info *ProgramInfo) *Program {
	return &Program{
		Process: kproc.ProcCommand(cmd),
		Status:  ST_PENDING,
		Sig:     make(chan os.Signal),
		info:    info,
	}
}

func (p *Program) createLog() (*os.File, error) {
	logDir := os.ExpandEnv("$HOME/.gosuv/logs")
	os.MkdirAll(logDir, 0755) // just do it, err ignore it
	logFile := filepath.Join(logDir, p.info.Name+".output.log")
	return os.Create(logFile)
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

func ServeAddr(host string, port int) error {
	r := mux.NewRouter()
	r.HandleFunc("/api/version", versionHandler)
	r.Methods("POST").Path("/api/shutdown").HandlerFunc(shutdownHandler)
	r.Methods("POST").Path("/api/programs").HandlerFunc(addHandler)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), r)
}
