package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/gorilla/mux"
	"github.com/qiniu/log"
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
	go program.Run()
	/*
		if err = program.Start(); err != nil {
			http.Error(w, err.Error(), 503)
			return
		}

		go program.Wait()
	*/

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

func ServeAddr(host string, port int) error {
	InitServer()
	r := mux.NewRouter()
	r.HandleFunc("/api/version", versionHandler)
	r.Methods("POST").Path("/api/shutdown").HandlerFunc(shutdownHandler)
	r.Methods("POST").Path("/api/programs").HandlerFunc(addHandler)
	r.Methods("GET").Path("/api/programs").HandlerFunc(statusHandler)
	return http.ListenAndServe(fmt.Sprintf("%s:%d", host, port), r)
}
