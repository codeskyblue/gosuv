package main

import (
	"encoding/json"
	"errors"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"sync"
	"syscall"
	"time"

	"github.com/codeskyblue/kproc"
	"github.com/qiniu/log"
)

var ErrGoTimeout = errors.New("GoTimeoutFunc")

func GoFunc(f func() error) chan error {
	ch := make(chan error)
	go func() {
		ch <- f()
	}()
	return ch
}

func GoTimeoutFunc(timeout time.Duration, f func() error) chan error {
	ch := make(chan error)
	go func() {
		var err error
		select {
		case err = <-GoFunc(f):
			ch <- err
		case <-time.After(timeout):
			log.Debugf("timeout: %v", f)
			ch <- ErrGoTimeout
		}
	}()
	return ch
}

const (
	ST_RUNNING   = "RUNNING"
	ST_STOPPED   = "STOPPED"
	ST_FATAL     = "FATAL"
	ST_RETRYWAIT = "RETRYWAIT" // some like python-supervisor EXITED
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
	AutoStart    bool     `json:"autostart"` // change to *bool, which support unexpected
	StartRetries int      `json:"startretries"`
	StartSeconds int      `json:"startsecs"`
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

	retry int
	stopc chan bool
}

func NewProgram(info *ProgramInfo) *Program {
	// set default values
	if info.StartRetries == 0 {
		info.StartRetries = 3
	}
	if info.StartSeconds == 0 {
		info.StartSeconds = 3
	}
	return &Program{
		//Process: kproc.ProcCommand(cmd),
		Status: ST_STOPPED,
		Sig:    make(chan os.Signal),
		Info:   info,
		stopc:  make(chan bool),
	}
}

func (p *Program) setStatus(st string) {
	// TODO: status change hook
	p.Status = st
}

func (p *Program) InputData(event Event) {
	switch event {
	case EVENT_START:
		if p.Status == ST_STOPPED || p.Status == ST_FATAL {
			go p.RunWithRetry()
		}
	case EVENT_STOP:
		if p.Status == ST_RUNNING || p.Status == ST_RETRYWAIT {
			p.Stop()
		}
	}
}

func (p *Program) createLog() (*os.File, error) {
	logDir := filepath.Join(GOSUV_HOME, "logs")
	os.MkdirAll(logDir, 0755) // just do it, err ignore it
	logFile := p.logFilePath()
	return os.OpenFile(logFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0644)
}

func (p *Program) sleep(d time.Duration) {
	select {
	case <-p.stopc:
		return
	case <-time.After(time.Second * 2):
	}
}

func (p *Program) logFilePath() string {
	logDir := filepath.Join(GOSUV_HOME, "logs")
	return filepath.Join(logDir, p.Info.Name+".log")
}

func (p *Program) RunWithRetry() {
	for p.retry = 0; p.retry < p.Info.StartRetries+1; p.retry += 1 {
		// wait program to exit
		errc := GoFunc(p.Run)
		var err error

	PROGRAM_WAIT:
		// Here is RUNNING State
		select {
		case err = <-errc:
			log.Info(p.Info.Name, err)
		case <-time.After(time.Second * time.Duration(p.Info.StartSeconds)): // reset retry
			p.retry = 0
			goto PROGRAM_WAIT
		case <-p.stopc:
			return
		}

		// Enter RETRY_WAIT State
		if p.retry < p.Info.StartRetries {
			p.setStatus(ST_RETRYWAIT)
			select {
			case <-p.stopc:
				return
			case <-time.After(time.Second * 2):
			}
		}
	}
	p.setStatus(ST_FATAL)
}

func (p *Program) Run() (err error) {
	if err = p.Start(); err != nil {
		return
	}
	p.setStatus(ST_RUNNING)
	defer func() {
		if out, ok := p.Cmd.Stdout.(io.Closer); ok {
			out.Close()
		}
		log.Warnf("program finish: %v", err)
	}()
	err = p.Wait()
	return
}

func (p *Program) Stop() error {
	select {
	case p.stopc <- true: // stopc may not recevied
	case <-time.After(time.Millisecond * 50):
	}
	p.Terminate(syscall.SIGKILL)
	p.setStatus(ST_STOPPED)
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
	names := []string{}
	for name, _ := range pt.table {
		names = append(names, name)
	}
	// log.Println(names)
	sort.Strings(names)
	// log.Println(names)
	for _, name := range names {
		ps = append(ps, pt.table[name])
	}
	// for _, p := range pt.table {
	// ps = append(ps, p)
	// }
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
