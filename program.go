package main

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

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
	ST_PENDING = "pending"
	ST_RUNNING = "running"
	ST_STOPPED = "stopped"
	ST_FATAL   = "fatal"
)

type Program struct {
	*kproc.Process `json:"-"`
	Status         string         `json:"state"`
	Sig            chan os.Signal `json:"-"`
	Info           *ProgramInfo   `json:"info"`
}

func NewProgram(cmd *exec.Cmd, info *ProgramInfo) *Program {
	return &Program{
		Process: kproc.ProcCommand(cmd),
		Status:  ST_PENDING,
		Sig:     make(chan os.Signal),
		Info:    info,
	}
}

func (p *Program) createLog() (*os.File, error) {
	logDir := os.ExpandEnv("$HOME/.gosuv/logs")
	os.MkdirAll(logDir, 0755) // just do it, err ignore it
	logFile := filepath.Join(logDir, p.Info.Name+".output.log")
	return os.Create(logFile)
}

func (p *Program) Run() error {
	if err := p.Start(); err != nil {
		p.Status = ST_FATAL
		return err
	}
	return p.Wait()
}

func (p *Program) Start() error {
	logFd, err := p.createLog()
	if err != nil {
		return err
	}
	p.Cmd.Stdout = logFd
	p.Cmd.Stderr = logFd
	return p.Cmd.Start()
}

// wait func finish, also accept signal
func (p *Program) Wait() (err error) {
	log.Println("Wait program to finish")
	p.Status = ST_RUNNING
	defer func() {
		if out, ok := p.Cmd.Stdout.(io.Closer); ok {
			out.Close()
		}
		if err != nil {
			log.Warnf("program finish: %v", err)
			p.Status = ST_FATAL
		} else {
			p.Status = ST_STOPPED
		}
	}()
	ch := GoFunc(p.Cmd.Wait)
	for {
		select {
		case err = <-ch:
			return err
		case sig := <-p.Sig:
			p.Terminate(sig)
		}
	}
}

type ProgramInfo struct {
	Name    string   `json:"name"`
	Command []string `json:"command"`
	Dir     string   `json:"dir"`
	Environ []string `json:"environ"`
}

var programTable *ProgramTable

func init() {
	programTable = &ProgramTable{
		table: make(map[string]*Program, 10),
		ch:    make(chan string),
	}
}

type ProgramTable struct {
	table map[string]*Program
	ch    chan string
	mu    sync.Mutex
}

var (
	ErrProgramDuplicate = errors.New("program duplicate")
)

func (pt *ProgramTable) AddProgram(p *Program) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()
	name := p.Info.Name
	if _, exists := pt.table[name]; exists {
		return ErrProgramDuplicate
	}
	pt.table[name] = p
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
