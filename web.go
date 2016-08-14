package main

import (
	"encoding/json"
	"html/template"
	"net/http"

	"github.com/gorilla/mux"
)

type Supervisor struct {
	ConfigDir string
}

func (s *Supervisor) Index(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("t").ParseFiles("./res/index.html"))
	t.ExecuteTemplate(w, "index.html", nil)
}

func (s *Supervisor) AddProgram(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	data, _ := json.Marshal(map[string]interface{}{
		"status": 0,
	})
	w.Write(data)
}

func init() {
	suv := &Supervisor{}
	r := mux.NewRouter()
	r.HandleFunc("/", suv.Index)
	r.HandleFunc("/api/programs", suv.AddProgram).Methods("POST")
	http.Handle("/", r)
}
