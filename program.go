package main

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"sort"
	"sync"

	. "github.com/codeskyblue/gosuv/config"
	. "github.com/codeskyblue/gosuv/program"
)

var programTable *ProgramTable

func initProgramTable() {
	programTable = &ProgramTable{
		table: make(map[string]*Program, 10),
		ch:    make(chan string),
	}
	programTable.loadConfig()
}

type ProgramTable struct {
	table map[string]*Program
	ch    chan string
	mu    sync.RWMutex
}

var (
	ErrProgramDuplicate = errors.New("program duplicated")
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
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	ps := make([]*Program, 0, len(pt.table))
	names := []string{}
	for name, _ := range pt.table {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		ps = append(ps, pt.table[name])
	}
	return ps
}

// golang map get will alwyas be safe.
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
		program.InputData(EVENT_STOP)
	}
}

func (pt *ProgramTable) Remove(name string) error {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	program, exists := pt.table[name]
	if !exists {
		return ErrProgramNotExists
	}
	program.InputData(EVENT_STOP)
	// program.Stop()
	delete(pt.table, name)
	return pt.saveConfig()
}
