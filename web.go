package main

import (
	"encoding/json"
	"html/template"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"reflect"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"
	"github.com/qiniu/log"
)

type Supervisor struct {
	ConfigDir string
	pgs       []*Program
	pgMap     map[string]*Program
	procMap   map[string]*Process
}

func (s *Supervisor) programPath() string {
	return filepath.Join(s.ConfigDir, "programs.yml")
}

func (s *Supervisor) addOrUpdateProgram(pg Program) {
	origPg, ok := s.pgMap[pg.Name]
	if ok {
		if !reflect.DeepEqual(origPg, &pg) {
			log.Println("Update:", pg.Name)
			origProc := s.procMap[pg.Name]
			isRunning := origProc.IsRunning()
			go func() {
				origProc.Operate(StopEvent)

				// TODO: wait state change
				time.Sleep(2 * time.Second)

				newProc := NewProcess(pg)
				s.procMap[pg.Name] = newProc
				if isRunning {
					newProc.Operate(StartEvent)
				}
			}()
		}
	} else {
		s.pgs = append(s.pgs, &pg)
		s.pgMap[pg.Name] = &pg
		s.procMap[pg.Name] = NewProcess(pg)
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
	// add or update program
	visited := map[string]bool{}
	for _, pg := range pgs {
		if visited[pg.Name] {
			log.Warnf("Duplicated program name: %s", pg.Name)
			continue
		}
		visited[pg.Name] = true
		s.addOrUpdateProgram(pg)
	}
	// delete not exists program
	for _, pg := range s.pgs {
		if visited[pg.Name] {
			continue
		}
		name := pg.Name
		s.procMap[name].Operate(StopEvent)
		delete(s.procMap, name)
		delete(s.pgMap, name)
	}
	// update programs (because of delete)
	s.pgs = make([]*Program, 0, len(s.pgMap))
	for _, pg := range s.pgMap {
		s.pgs = append(s.pgs, pg)
	}
	return nil
}

func (s *Supervisor) saveDB() error {
	data, err := yaml.Marshal(s.pgs)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.programPath(), data, 0644)
}

func (s *Supervisor) Index(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("t").ParseFiles("./res/index.html"))
	t.ExecuteTemplate(w, "index.html", nil)
}

func (s *Supervisor) AddProgram(w http.ResponseWriter, r *http.Request) {
	pg := Program{
		Name:    r.FormValue("name"),
		Command: r.FormValue("command"),
		// TODO: missing other values
	}
	if err := pg.Check(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
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

	fs := http.FileServer(http.Dir("res"))
	http.Handle("/", r)
	http.Handle("/res/", http.StripPrefix("/res/", fs))
}
