package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/codeskyblue/kproc"
	"github.com/qiniu/log"
)

func GoFunc(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

const (
	ST_STANDBY   = "STANDBY"
	ST_RUNNING   = "RUNNING"
	ST_STOPPED   = "STOPPED"
	ST_FATAL     = "FATAL"
	ST_RETRYWAIT = "RETRYWAIT"
)

type Event int

const (
	EVENT_START = Event(iota)
	EVENT_STOP
)

type ProgramInfo struct {
	Name         string   `json:"name"`
	Command      []string `json:"command"`
	Dir          string   `json:"dir"`
	Environ      []string `json:"environ"`
	AutoStart    bool     `json:"autostart"`
	StartRetries int      `json:"startretries"`
}

func (p *ProgramInfo) buildCmd() *exec.Cmd {
	cmd := exec.Command(p.Command[0], p.Command[1:]...)
	cmd.Dir = p.Dir
	cmd.Env = append(os.Environ(), p.Environ...)
	return cmd
}

type Program struct {
	*kproc.Process `json:"-"`
	Status         string         `json:"state"`
	Sig            chan os.Signal `json:"-"`
	Info           *ProgramInfo   `json:"info"`

	retry   int
	stopped bool
}

func NewProgram(info *ProgramInfo) *Program {
	// set default values
	if info.StartRetries == 0 {
		info.StartRetries = 3
	}
	return &Program{
		//Process: kproc.ProcCommand(cmd),
		Status: ST_STANDBY,
		Sig:    make(chan os.Signal),
		Info:   info,
	}
}

func (p *Program) setStatus(st string) {
	// TODO: status change hook
	p.Status = st
}

func (p *Program) InputData(event Event) {
	switch event {
	case EVENT_START:
		if p.Status != ST_RUNNING {
			go p.RunWithRetry()
		}
	case EVENT_STOP:
		if p.Status == ST_RUNNING {
			p.Stop()
		}
	}
}

func (p *Program) createLog() (*os.File, error) {
	logDir := filepath.Join(GOSUV_HOME, "logs")
	os.MkdirAll(logDir, 0755) // just do it, err ignore it
	logFile := filepath.Join(logDir, p.Info.Name+".output.log")
	return os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
}

func (p *Program) sleep(d time.Duration) {
	// FIXME(ssx): when signal comes, finished sleep
	time.Sleep(d)
}

func (p *Program) RunWithRetry() {
	p.stopped = false
	for p.retry = 0; p.retry < p.Info.StartRetries; p.retry += 1 {
		p.Run()
		if p.stopped {
			p.setStatus(ST_STOPPED)
			return
		}
		if p.retry+1 < p.Info.StartRetries {
			p.setStatus(ST_RETRYWAIT)
			p.sleep(time.Second * 2) // RETRYWAIT
		}
	}
	p.setStatus(ST_FATAL)
}

func (p *Program) Run() (err error) {
	if err := p.Start(); err != nil {
		log.Println("start:", err)
		p.setStatus(ST_FATAL)
		return err
	}
	p.setStatus(ST_RUNNING)
	defer func() {
		if out, ok := p.Cmd.Stdout.(io.Closer); ok {
			out.Close()
		}
		if !p.stopped && err != nil {
			log.Warnf("program finish: %v", err)
			p.setStatus(ST_FATAL)
		} else {
			p.setStatus(ST_STOPPED)
		}
	}()
	err = p.Wait()
	return
}

func (p *Program) Stop() error {
	p.stopped = true
	p.Terminate(syscall.SIGKILL)
	return nil
}

func (p *Program) Start() error {
	p.Process = kproc.ProcCommand(p.Info.buildCmd())
	logFd, err := p.createLog()
	if err != nil {
		return err
	}
	p.Cmd.Stdout = logFd
	p.Cmd.Stderr = logFd
	return p.Cmd.Start()
}

var programTable *ProgramTable

func InitServer() {
	programTable = &ProgramTable{
		table: make(map[string]*Program, 10),
		ch:    make(chan string),
	}
	programTable.loadConfig()
}

type ProgramTable struct {
	table map[string]*Program
	ch    chan string
	mu    sync.Mutex
}

var (
	ErrProgramDuplicate = errors.New("program duplicate")
	ErrProgramNotExists = errors.New("program not exists")
)

func (pt *ProgramTable) saveConfig() error {
	table := make(map[string]*ProgramInfo)
	for name, p := range pt.table {
		table[name] = p.Info
	}
	cfgFd, err := os.Create(GOSUV_PROGRAM_CONFIG)
	if err != nil {
		return err
	}
	defer cfgFd.Close()
	data, _ := json.MarshalIndent(table, "", "    ")
	return ioutil.WriteFile(GOSUV_PROGRAM_CONFIG, data, 0644)
}

func (pt *ProgramTable) loadConfig() error {
	cfgFd, err := os.Open(GOSUV_PROGRAM_CONFIG)
	if err != nil {
		return err
	}
	defer cfgFd.Close()
	table := make(map[string]*ProgramInfo)
	if err = json.NewDecoder(cfgFd).Decode(&table); err != nil {
		return err
	}
	for name, pinfo := range table {
		program := NewProgram(pinfo)
		pt.table[name] = program
		if pinfo.AutoStart {
			program.InputData(EVENT_START)
		}
	}
	return nil
}

func (pt *ProgramTable) AddProgram(p *Program) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	name := p.Info.Name
	if _, exists := pt.table[name]; exists {
		return ErrProgramDuplicate
	}
	pt.table[name] = p
	pt.saveConfig()
	return nil
}

func (pt *ProgramTable) Programs() []*Program {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	ps := make([]*Program, 0, len(pt.table))
	for _, p := range pt.table {
		ps = append(ps, p)
	}
	return ps
}

func (pt *ProgramTable) Get(name string) (*Program, error) {
	program, exists := pt.table[name]
	if !exists {
		return nil, ErrProgramNotExists
	}
	return program, nil
}

func (pt *ProgramTable) StopAll() {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	for _, program := range pt.table {
		program.Stop()
	}
}
