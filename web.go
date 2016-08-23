package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
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

func (s *Supervisor) addOrUpdateProgram(pg Program) error {
	origPg, ok := s.pgMap[pg.Name]
	if ok {
		// log.Println("Orig:", origPg, "Curr:", pg)
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
	return s.saveDB()
}

// Check
// - Yaml format
// - Duplicated program
func (s *Supervisor) readConfigFromDB() (pgs []Program, err error) {
	data, err := ioutil.ReadFile(s.programPath())
	if err != nil {
		data = []byte("")
	}
	pgs = make([]Program, 0)
	if err = yaml.Unmarshal(data, &pgs); err != nil {
		return nil, err
	}
	visited := map[string]bool{}
	for _, pg := range pgs {
		if visited[pg.Name] {
			return nil, fmt.Errorf("Duplicated program name: %s", pg.Name)
		}
		visited[pg.Name] = true
	}
	return
}

func (s *Supervisor) loadDB() error {
	pgs, err := s.readConfigFromDB()
	if err != nil {
		return err
	}
	// add or update program
	visited := map[string]bool{}
	for _, pg := range pgs {
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
	dir := filepath.Dir(s.programPath())
	if !IsDir(dir) {
		os.MkdirAll(dir, 0755)
	}

	data, err := yaml.Marshal(s.pgs)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(s.programPath(), data, 0644)
}

func (s *Supervisor) hIndex(w http.ResponseWriter, r *http.Request) {
	t := template.Must(template.New("t").ParseFiles("./res/index.html"))
	t.ExecuteTemplate(w, "index.html", nil)
}

func (s *Supervisor) hGetProgram(w http.ResponseWriter, r *http.Request) {
	procs := make([]*Process, 0, len(s.pgs))
	for _, pg := range s.pgs {
		procs = append(procs, s.procMap[pg.Name])
	}
	log.Println(procs[0])
	data, err := json.Marshal(procs)
	log.Println(string(data))
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Supervisor) hAddProgram(w http.ResponseWriter, r *http.Request) {
	pg := Program{
		Name:      r.FormValue("name"),
		Command:   r.FormValue("command"),
		Dir:       r.FormValue("dir"),
		AutoStart: r.FormValue("autostart") == "on",
		// TODO: missing other values
	}
	if pg.Dir == "" {
		pg.Dir = "/"
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
			"error":  fmt.Sprintf("Program %s already exists", strconv.Quote(pg.Name)),
		})
	} else {
		if err := s.addOrUpdateProgram(pg); err != nil {
			data, _ = json.Marshal(map[string]interface{}{
				"status": 1,
				"error":  err.Error(),
			})
		} else {
			data, _ = json.Marshal(map[string]interface{}{
				"status": 0,
			})
		}
	}
	w.Write(data)
}

func init() {
	suv := &Supervisor{
		ConfigDir: filepath.Join(UserHomeDir(), ".gosuv"),
		pgMap:     make(map[string]*Program, 0),
		procMap:   make(map[string]*Process, 0),
	}
	if err := suv.loadDB(); err != nil {
		log.Fatal(err)
	}
	r := mux.NewRouter()
	r.HandleFunc("/", suv.hIndex)
	r.HandleFunc("/api/programs", suv.hGetProgram).Methods("GET")
	r.HandleFunc("/api/programs", suv.hAddProgram).Methods("POST")

	fs := http.FileServer(http.Dir("res"))
	http.Handle("/", r)
	http.Handle("/res/", http.StripPrefix("/res/", fs))
}
