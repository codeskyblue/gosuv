package main

import (
	"encoding/json"
	"fmt"
	"html/template"
	"io/ioutil"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"reflect"
	"strconv"
	"sync"
	"syscall"
	"time"

	"github.com/go-yaml/yaml"
	"github.com/gorilla/mux"
	"github.com/gorilla/websocket"
	"github.com/qiniu/log"
)

type Supervisor struct {
	ConfigDir string
	pgs       []*Program
	pgMap     map[string]*Program
	procMap   map[string]*Process
	eventCs   []chan string // channels
	mu        sync.Mutex
}

func (s *Supervisor) programPath() string {
	return filepath.Join(s.ConfigDir, "programs.yml")
}

func (s *Supervisor) newProcess(pg Program) *Process {
	p := NewProcess(pg)
	origFunc := p.StateChange
	p.StateChange = func(oldState, newState FSMState) {
		s.broadcastEvent(string(newState))
		origFunc(oldState, newState)
	}
	return p
}

func (s *Supervisor) broadcastEvent(event string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	validEventCs := make([]chan string, 0, len(s.eventCs))
	for _, c := range s.eventCs {
		select {
		case c <- event:
			validEventCs = append(validEventCs, c)
		case <-time.After(500 * time.Millisecond):
			log.Println("Chan closed, remove from queue")
		}
	}
	s.eventCs = validEventCs
}

func (s *Supervisor) addOrUpdateProgram(pg Program) error {
	defer s.broadcastEvent("add or update")

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

				newProc := s.newProcess(pg)
				s.procMap[pg.Name] = newProc
				if isRunning {
					newProc.Operate(StartEvent)
				}
			}()
		}
	} else {
		s.pgs = append(s.pgs, &pg)
		s.pgMap[pg.Name] = &pg
		s.procMap[pg.Name] = s.newProcess(pg)
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
	data, err := json.Marshal(procs)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.Write(data)
}

func (s *Supervisor) hAddProgram(w http.ResponseWriter, r *http.Request) {
	retries, err := strconv.Atoi(r.FormValue("retries"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	pg := Program{
		Name:         r.FormValue("name"),
		Command:      r.FormValue("command"),
		Dir:          r.FormValue("dir"),
		AutoStart:    r.FormValue("autostart") == "on",
		StartRetries: retries,
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

func (s *Supervisor) hStartProgram(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	proc, ok := s.procMap[name]
	var data []byte
	if !ok {
		data, _ = json.Marshal(map[string]interface{}{
			"status": 1,
			"error":  fmt.Sprintf("Process %s not exists", strconv.Quote(name)),
		})
	} else {
		proc.Operate(StartEvent)
		data, _ = json.Marshal(map[string]interface{}{
			"status": 0,
			"name":   name,
		})
	}
	w.Write(data)
}

func (s *Supervisor) hStopProgram(w http.ResponseWriter, r *http.Request) {
	name := mux.Vars(r)["name"]
	proc, ok := s.procMap[name]
	var data []byte
	if !ok {
		data, _ = json.Marshal(map[string]interface{}{
			"status": 1,
			"error":  fmt.Sprintf("Process %s not exists", strconv.Quote(name)),
		})
	} else {
		proc.Operate(StopEvent)
		data, _ = json.Marshal(map[string]interface{}{
			"status": 0,
			"name":   name,
		})
	}
	w.Write(data)
}

var upgrader = websocket.Upgrader{}

func (s *Supervisor) wsEvents(w http.ResponseWriter, r *http.Request) {
	c, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Print("upgrade:", err)
		return
	}
	defer c.Close()

	ch := make(chan string, 0)
	s.eventCs = append(s.eventCs, ch)
	go func() {
		for message := range ch {
			// Question: type 1 ?
			c.WriteMessage(1, []byte(message))
		}
		close(ch)
	}()
	for {
		mt, message, err := c.ReadMessage()
		if err != nil {
			log.Println("read:", mt, err)
			break
		}
		log.Printf("recv: %v %s", mt, message)
		err = c.WriteMessage(mt, message)
		if err != nil {
			log.Println("write:", err)
			break
		}
	}
}

func (s *Supervisor) catchExitSignal() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		sig := <-c
		fmt.Printf("Got signal: %v, stopping all running process\n", sig)
		for _, proc := range s.procMap {
			proc.stopCommand()
		}
		fmt.Println("Finished. Exit with code 0")
		os.Exit(0)
	}()
}

func init() {
	suv := &Supervisor{
		ConfigDir: filepath.Join(UserHomeDir(), ".gosuv"),
		pgMap:     make(map[string]*Program, 0),
		procMap:   make(map[string]*Process, 0),
		eventCs:   make([]chan string, 0),
	}
	if err := suv.loadDB(); err != nil {
		log.Fatal(err)
	}
	suv.catchExitSignal()

	r := mux.NewRouter()
	r.HandleFunc("/", suv.hIndex)
	r.HandleFunc("/api/programs", suv.hGetProgram).Methods("GET")
	r.HandleFunc("/api/programs", suv.hAddProgram).Methods("POST")
	r.HandleFunc("/api/programs/{name}/start", suv.hStartProgram).Methods("POST")
	r.HandleFunc("/api/programs/{name}/stop", suv.hStopProgram).Methods("POST")
	r.HandleFunc("/ws/events", suv.wsEvents)

	fs := http.FileServer(http.Dir("res"))
	http.Handle("/", r)
	http.Handle("/res/", http.StripPrefix("/res/", fs))
}
