package main

import (
	"log"
	"os"
	"sync"
	"syscall"
	"time"

	"github.com/codeskyblue/kexec"
)

type FSMState string
type FSMEvent string
type FSMHandler func()

type FSM struct {
	mu       sync.Mutex
	state    FSMState
	handlers map[FSMState]map[FSMEvent]FSMHandler
}

func (f *FSM) AddHandler(state FSMState, event FSMEvent, hdlr FSMHandler) *FSM {
	_, ok := f.handlers[state]
	if !ok {
		f.handlers[state] = make(map[FSMEvent]FSMHandler)
	}
	if _, ok = f.handlers[state][event]; ok {
		log.Fatalf("set twice for state(%s) event(%s)", state, event)
	}
	f.handlers[state][event] = hdlr
	return f
}

func (f *FSM) State() FSMState {
	return f.state
}

func (f *FSM) SetState(newState FSMState) {
	f.state = newState
}

func (f *FSM) Operate(event FSMEvent) FSMState {
	f.mu.Lock()
	defer f.mu.Unlock()

	eventMap := f.handlers[f.State()]
	if eventMap == nil {
		return f.State()
	}
	if fn, ok := eventMap[event]; ok {
		fn()
	}
	return f.State()
}

func NewFSM(initState FSMState) *FSM {
	return &FSM{
		state:    initState,
		handlers: make(map[FSMState]map[FSMEvent]FSMHandler),
	}
}

// Only 4 states now is enough, I think
var (
	Running   = FSMState("running")
	Stopped   = FSMState("stopped")
	Fatal     = FSMState("fatal")
	RetryWait = FSMState("retry wait")

	StartEvent   = FSMEvent("start")
	StopEvent    = FSMEvent("stop")
	RestartEvent = FSMEvent("restart")
)

type Program struct {
	Name         string   `yaml:"name"`
	Command      string   `yaml:"command"`
	Environ      []string `yaml:"environ"`
	Dir          string   `yaml:"directory"`
	AutoStart    bool     `yaml:"autostart"` // change to *bool, which support unexpected
	StartRetries int      `yaml:"startretries"`
	StartSeconds int      `yaml:"startsecs"`
	LogDir       string   `yaml:"logdir"`
}

type Process struct {
	*FSM
	Program
	cmd       *kexec.KCommand
	retryLeft int
}

func (p *Process) buildCommand() *kexec.KCommand {
	cmd := kexec.CommandString(p.Command) // Not tested here, I think it should work
	// cmd := kexec.Command(p.Command[0], p.Command[1:]...)
	cmd.Dir = p.Dir
	cmd.Env = append(os.Environ(), p.Environ...)
	return cmd
}

func (p *Process) waitNextRetry() {
	p.SetState(RetryWait)
	if p.retryLeft <= 0 {
		p.retryLeft = p.StartRetries
		p.SetState(Fatal)
		return
	}
	p.retryLeft -= 1
	select {
	case <-time.After(2 * time.Second): // TODO: need put it into Program
		go p.Operate(StartEvent)
	}
}

func (p *Process) waitExit() {
	select {
	case <-GoFunc(p.cmd.Wait):
	}
}

func NewProcess(pg Program) *Process {
	pr := &Process{
		FSM:       NewFSM(Stopped),
		Program:   pg,
		retryLeft: pg.StartRetries,
	}

	startFunc := func() {
		pr.SetState(Running)
		pr.cmd = kexec.CommandString("echo hello world && sleep 10 && echo end")
		pr.cmd.Stdout = os.Stdout
		go func() {
			errC := GoFunc(pr.cmd.Run)
			select {
			case err := <-errC: //<-GoTimeoutFunc(time.Duration(pr.StartSeconds)*time.Second, pr.cmd.Run):
				log.Println(err)
				pr.waitNextRetry()
				return
			case <-time.After(time.Duration(pr.StartSeconds) * time.Second):
				pr.retryLeft = pr.StartRetries // reset retries if success
			}
			// wait until exit
			select {
			case err := <-errC:
				log.Println(err)
				pr.waitNextRetry()
			}
		}()
	}

	pr.AddHandler(Stopped, StartEvent, startFunc)
	pr.AddHandler(Fatal, StartEvent, startFunc)

	pr.AddHandler(Running, StopEvent, func() {
		pr.cmd.Terminate(syscall.SIGKILL)
	}).AddHandler(Stopped, RestartEvent, func() {
		go pr.Operate(StartEvent)
	}).AddHandler(Running, RestartEvent, func() {
		go func() {
			pr.Operate(StopEvent)
			// TODO: start laterly
			time.Sleep(1 * time.Second)
			pr.Operate(StartEvent)
		}()
	})
	return pr
}

func init() {
	pg := Program{
		Name:    "demo",
		Command: "echo hello world && sleep 1 && echo end",
	}
	proc := NewProcess(pg)
	log.Println(proc.State())

	proc.Operate(RestartEvent)
	log.Println(proc.State())
	time.Sleep(2 * time.Second)
	log.Println(proc.State())
	proc.Operate(StopEvent)
	time.Sleep(1 * time.Second)
	log.Println(proc.State())
	// log.Println(light.State)
	// light.AddHandler(Opened, Close, func() {
	// 	log.Println("Close light")
	// 	light.State = Closed
	// })
	// light.Operate(Close)
	// log.Println(light.State)
}
