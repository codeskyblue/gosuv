package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"reflect"

	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"
)

type Supervisor struct {
	ConfigDir string
	pgs       []*Program
	pgMap     map[string]*Program
}

func (s *Supervisor) programPath() string {
	return filepath.Join(s.ConfigDir, "programs.yml")
}

func (s *Supervisor) addOrUpdateProgram(pg Program) {
	origPg, ok := s.pgMap[pg.Name]
	if ok {
		if !reflect.DeepEqual(origPg, &pg) {
			log.Println("Update:", pg.Name)
		}
	} else {
		s.pgs = append(s.pgs, &pg)
		s.pgMap[pg.Name] = &pg
		log.Println("Add:", pg.Name)
	}
}

func (s *Supervisor) loadDB() error {
	data, err := ioutil.ReadFile(s.programPath())
	if err != nil {
		data = []byte("")
	}
	pgs := make([]Program, 0)
	if err = yaml.Unmarshal(data, pgs); err != nil {
		return err
	}
	for _, pg := range pgs {
		s.addOrUpdateProgram(pg)
	}
	return nil
}

func (s *Supervisor) Index(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("t").ParseFiles("./res/index.html"))
	t.ExecuteTemplate(w, "index.html", nil)
}

func (s *Supervisor) AddProgram(w http.ResponseWriter, r *http.Request) {
	pg := Program{
		Name:    r.FormValue("name"),
		Command: r.FormValue("command"),
	}
	w.Header().Set("Content-Type", "application/json")
	var data []byte
	if _, ok := s.pgMap[pg.Name]; ok {
		data, _ = json.Marshal(map[string]interface{}{
			"status": 1,
		})
	} else {
		s.addOrUpdateProgram(pg)

		data, _ = json.Marshal(map[string]interface{}{
			"status": 0,
		})
	}
	w.Write(data)
}

func init() {
	suv := &Supervisor{}
	r := mux.NewRouter()
	r.HandleFunc("/", suv.Index)
	r.HandleFunc("/api/programs", suv.AddProgram).Methods("POST")
	http.Handle("/", r)
}
