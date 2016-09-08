// 研究了一天的状态机，天气也热，正当我写的即将昏迷之际，我突然醒悟了，原来状态机是这么一回事
// - 状态比喻成 数据结构
// - 事件比喻成 用户输入
// - 状态转移则是函数调用
// 如此依赖写成函数，也就是 (Orz 原来如此)
// type FSM struct {
// 	State 			FSMState
// 	TransformFuncs  map[FSMState] func()
// }

// func (f *FSM) UserAction(action FSMAction) {
// 	...
// }

package main

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	"github.com/codeskyblue/gosuv/pushover"
	"github.com/codeskyblue/kexec"
	"github.com/kennygrant/sanitize"
	"github.com/qiniu/log"
)

type FSMState string
type FSMEvent string
type FSMHandler func()

type FSM struct {
	mu       sync.Mutex
	state    FSMState
	handlers map[FSMState]map[FSMEvent]FSMHandler

	StateChange func(oldState, newState FSMState)
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
	if f.StateChange != nil {
		f.StateChange(f.state, newState)
	}
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
	Name          string   `yaml:"name" json:"name"`
	Command       string   `yaml:"command" json:"command"`
	Environ       []string `yaml:"environ" json:"environ"`
	Dir           string   `yaml:"directory" json:"directory"`
	StartAuto     bool     `yaml:"start_auto" json:"startAuto"`
	StartRetries  int      `yaml:"start_retries" json:"startRetries"`
	StartSeconds  int      `yaml:"start_seconds,omitempty" json:"startSeconds"`
	User          string   `yaml:"user,omitempty" json:"user"`
	Notifications struct {
		Pushover struct {
			ApiKey string   `yaml:"api_key"`
			Users  []string `yaml:"users"`
		} `yaml:"pushover,omitempty"`
	} `yaml:"notifications,omitempty" json:"-"`
	WebHook struct {
		Github struct {
			Secret string `yaml:"secret"`
		} `yaml:"github"`
		Command string `yaml:"command"`
		Timeout int    `yaml:"timeout"`
	} `yaml:"webhook,omitempty" json:"-"`
}

func (p *Program) Check() error {
	if p.Name == "" {
		return errors.New("Program name empty")
	}
	if p.Command == "" {
		return errors.New("Program command empty")
	}
	if p.Dir != "" && !IsDir(p.Dir) {
		return fmt.Errorf("Program dir(%s) not exists", p.Dir)
	}
	return nil
}

func (p *Program) RunNotification() {
	po := p.Notifications.Pushover
	if po.ApiKey != "" && len(po.Users) > 0 {
		for _, user := range po.Users {
			err := pushover.Notify(pushover.Params{
				Token:   po.ApiKey,
				User:    user,
				Title:   "gosuv",
				Message: fmt.Sprintf("%s change to fatal", p.Name),
			})
			if err != nil {
				log.Warnf("pushover error: %v", err)
			}
		}
	}
}

type Process struct {
	*FSM       `json:"-"`
	Program    `json:"program"`
	cmd        *kexec.KCommand
	Stdout     *QuickLossBroadcastWriter `json:"-"`
	Stderr     *QuickLossBroadcastWriter `json:"-"`
	Output     *QuickLossBroadcastWriter `json:"-"`
	OutputFile *os.File                  `json:"-"`
	stopC      chan syscall.Signal
	retryLeft  int
	Status     string `json:"status"`
}

// FIXME(ssx): maybe need to return error
func (p *Process) buildCommand() *kexec.KCommand {
	cmd := kexec.CommandString(p.Command)
	// cmd := kexec.Command(p.Command[0], p.Command[1:]...)
	cmd.Dir = p.Dir
	if p.User != "" {
		if err := cmd.SetUser(p.User); err != nil {
			log.Warnf("cmd:%s chusr to %s failed", p.Name, p.User)
		}
	}
	logDir := filepath.Join(defaultConfigDir, "log", sanitize.Name(p.Name))
	if !IsDir(logDir) {
		os.MkdirAll(logDir, 0755)
	}
	var fout io.Writer
	var err error
	p.OutputFile, err = os.OpenFile(filepath.Join(logDir, "output.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		log.Warn("create stdout log failed:", err)
		fout = ioutil.Discard
	} else {
		fout = p.OutputFile
	}
	cmd.Stdout = io.MultiWriter(p.Stdout, p.Output, fout)
	cmd.Stderr = io.MultiWriter(p.Stderr, p.Output, fout)
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
		p.startCommand()
	case <-p.stopC:
		p.stopCommand()
	}
}

func (p *Process) stopCommand() {
	if p.cmd == nil {
		return
	}
	p.cmd.Terminate(syscall.SIGTERM)
	select {
	case <-GoFunc(p.cmd.Wait):
	case <-time.After(3 * time.Second): // TODO: add 3s to config
		p.cmd.Terminate(syscall.SIGKILL)
	}
	err := p.cmd.Wait() // This is OK, because Signal KILL will definitely work
	prefixStr := "\n--- GOSUV LOG " + time.Now().Format("2006-01-02 15:04:05")
	if err == nil {
		io.WriteString(p.cmd.Stderr, fmt.Sprintf("%s exit success ---\n\n", prefixStr))
	} else {
		io.WriteString(p.cmd.Stderr, fmt.Sprintf("%s exit %v ---\n\n", prefixStr, err))
	}
	if p.OutputFile != nil {
		p.OutputFile.Close()
		p.OutputFile = nil
	}
	p.cmd = nil
	p.SetState(Stopped)
}

func (p *Process) IsRunning() bool {
	return p.State() == Running || p.State() == RetryWait
}

func (p *Process) startCommand() {
	p.stopCommand()
	// p.Stdout.Reset()
	// p.Stderr.Reset()
	// p.Output.Reset() // Donot reset because log is still needed.
	log.Printf("start cmd(%s): %s", p.Name, p.Command)
	p.cmd = p.buildCommand()

	p.SetState(Running)
	go func() {
		errC := GoFunc(p.cmd.Run)
		startTime := time.Now()
		select {
		case err := <-errC:
			log.Println(err, time.Since(startTime))
			if time.Since(startTime) < time.Duration(p.StartSeconds)*time.Second {
				if p.retryLeft == p.StartRetries { // If first time quit so fast, just set to fatal
					p.SetState(Fatal)
					log.Println("Start change to fatal")
					return
				}
			}
			p.waitNextRetry()
		case <-p.stopC:
			log.Println("recv stop command")
			p.stopCommand()
		}
	}()
}

func NewProcess(pg Program) *Process {
	outputBufferSize := 24 * 1024 // 24K
	pr := &Process{
		FSM:       NewFSM(Stopped),
		Program:   pg,
		stopC:     make(chan syscall.Signal),
		retryLeft: pg.StartRetries,
		Status:    string(Stopped),
		Output:    NewQuickLossBroadcastWriter(outputBufferSize),
		Stdout:    NewQuickLossBroadcastWriter(outputBufferSize),
		Stderr:    NewQuickLossBroadcastWriter(outputBufferSize),
	}
	pr.StateChange = func(_, newStatus FSMState) {
		pr.Status = string(newStatus)

		// TODO: status need to filter with config, not hard coded.
		if newStatus == Fatal {
			go pr.Program.RunNotification()
		}
	}
	if pr.StartSeconds <= 0 {
		pr.StartSeconds = 3
	}

	pr.AddHandler(Stopped, StartEvent, func() {
		pr.retryLeft = pr.StartRetries
		pr.startCommand()
	})
	pr.AddHandler(Fatal, StartEvent, pr.startCommand)

	pr.AddHandler(Running, StopEvent, func() {
		select {
		case pr.stopC <- syscall.SIGTERM:
		case <-time.After(200 * time.Millisecond):
		}
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
	// pg := Program{
	// 	Name:    "demo",
	// 	Command: "echo hello world && sleep 1 && echo end",
	// }
	// proc := NewProcess(pg)
	// log.Println(proc.State())

	// proc.Operate(StartEvent)
	// log.Println(proc.State())
	// time.Sleep(2 * time.Second)
	// log.Println(proc.State())
	// proc.Operate(StopEvent)
	// time.Sleep(1 * time.Second)
	// log.Println(proc.State())
	// log.Println(light.State)
	// light.AddHandler(Opened, Close, func() {
	// 	log.Println("Close light")
	// 	light.State = Closed
	// })
	// light.Operate(Close)
	// log.Println(light.State)
}
