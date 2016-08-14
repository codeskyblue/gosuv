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

type ProcessFSM struct {
	*FSM

	Cmd        *kexec.KCommand
	retryCount int
	retryLeft  int
}

func NewProcessFSM() *ProcessFSM {
	return &ProcessFSM{
		FSM:        NewFSM(Stopped),
		retryCount: 3,
		retryLeft:  0,
	}
}

func init() {
	proc := NewProcessFSM()
	log.Println(proc.State())
	proc.AddHandler(Stopped, StartEvent, func() {
		proc.SetState(Running)
		proc.Cmd = kexec.CommandString("echo hello world && sleep 10 && echo end")
		proc.Cmd.Stdout = os.Stdout
		go func() {
			var err error
			select {
			case err = <-GoFunc(proc.Cmd.Run):
				log.Println(err)
				proc.SetState(Stopped)
			}
		}()
	}).AddHandler(Running, StopEvent, func() {
		proc.Cmd.Terminate(syscall.SIGKILL)
	}).AddHandler(Stopped, RestartEvent, func() {
		go proc.Operate(StartEvent)
	}).AddHandler(Running, RestartEvent, func() {
		go func() {
			proc.Operate(StopEvent)
			// 	// TODO: start laterly
			time.Sleep(1 * time.Second)
			proc.Operate(StartEvent)
		}()
	})

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
