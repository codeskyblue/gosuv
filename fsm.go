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
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/kennygrant/sanitize"
	"github.com/lunny/dingtalk_webhook"
	"github.com/natefinch/lumberjack"
	"github.com/qiniu/log"
	"github.com/soopsio/gosuv/pushover"
	"github.com/soopsio/kexec"
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
// 2016-09-18 now five
var (
	Running   = FSMState("running")
	Stopped   = FSMState("stopped")
	Fatal     = FSMState("fatal")
	RetryWait = FSMState("retry wait")
	Stopping  = FSMState("stopping")

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
	StopTimeout   int      `yaml:"stop_timeout,omitempty" json:"stopTimeout"`
	retryCount    int
	User          string        `yaml:"user,omitempty" json:"user"`
	Notifications Notifications `yaml:"notifications,omitempty" json:"-"`
	WebHook       WebHook       `yaml:"webhook,omitempty" json:"-"`
}

type Notifications struct {
	Pushover struct {
		ApiKey string   `yaml:"api_key"`
		Users  []string `yaml:"users"`
	} `yaml:"pushover,omitempty"`

	Dingtalk struct {
		Groups []struct {
			Secret  string   `yaml:"secret"`
			Mobiles []string `yaml:"mobile"`
		} `yaml:"groups"`
	} `yaml:"dingtalk,omitempty"`
}

type WebHook struct {
	Github struct {
		Secret string `yaml:"secret"`
	} `yaml:"github"`
	Command string `yaml:"command"`
	Timeout int    `yaml:"timeout"`
}

func (p *Program) Check() error {
	if p.Name == "" {
		return errors.New("Program name empty")
	}
	if p.Command == "" {
		return errors.New("Program command empty")
	}
	// Disable check, for Dir may contains env-vars
	//if p.Dir != "" && !IsDir(p.Dir) {
	//	return fmt.Errorf("Program dir(%s) not exists", p.Dir)
	//}
	return nil
}

func (p *Program) RunNotification(state FSMState) {
	notis := []Notifications{}
	notis = append(notis, cfg.Notifications)
	t := time.Now().Format("2006-01-02 15:04:05")
	host := ""
	if cfg.Server.Name != "" {
		host = cfg.Server.Name
	} else {
		host, _ = os.Hostname()
	}
	msg := fmt.Sprintf("[%s] %s: \"%s\" changed: \"%s\"", t, host, p.Name, state)
	if state == RetryWait {
		msg += " retryCount:" + strconv.Itoa(p.retryCount)
	}
	for _, noti := range notis {
		po := noti.Pushover
		if po.ApiKey != "" && len(po.Users) > 0 {
			for _, user := range po.Users {
				err := pushover.Notify(pushover.Params{
					Token:   po.ApiKey,
					User:    user,
					Title:   "gosuv",
					Message: msg,
				})
				if err != nil {
					log.Warnf("pushover error: %v", err)
				}
			}
		}

		pw := noti.Dingtalk
		if len(pw.Groups) > 0 {
			for _, group := range pw.Groups {
				ding := dingtalk.NewWebhook(group.Secret)
				err := ding.SendTextMsg(msg, false, group.Mobiles...)
				if err != nil {
					log.Error("钉钉通知失败:", err)
				}
			}
		}
	}
}

func IsRoot() bool {
	u, err := user.Current()
	return err == nil && u.Username == "root"
}

type Process struct {
	*FSM       `json:"-"`
	Program    `json:"program"`
	cmd        *kexec.KCommand
	Stdout     *QuickLossBroadcastWriter `json:"-"`
	Stderr     *QuickLossBroadcastWriter `json:"-"`
	Output     *QuickLossBroadcastWriter `json:"-"`
	OutputFile io.WriteCloser            `json:"-"`
	stopC      chan syscall.Signal
	retryLeft  int
	Status     string `json:"status"`

	mu sync.Mutex
}

// FIXME(ssx): maybe need to return error
func (p *Process) buildCommand() *kexec.KCommand {
	cmd := kexec.CommandString(p.Command)
	// cmd := kexec.Command(p.Command[0], p.Command[1:]...)
	logDir := filepath.Join(defaultGosuvDir, "log", sanitize.Name(p.Name))
	if !IsDir(logDir) {
		os.MkdirAll(logDir, 0755)
	}
	var fout io.Writer
	var err error
	// p.OutputFile, err = os.OpenFile(filepath.Join(logDir, "output.log"), os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	// p.OutputFile = NewRotate(filepath.Join(logDir, "output.log"))
	p.OutputFile = &lumberjack.Logger{
		Filename:   filepath.Join(logDir, "output.log"),
		MaxSize:    1024,
		MaxAge:     14,
		MaxBackups: 14,
		Compress:   false,
		LocalTime:  true,
	}

	if err != nil {
		log.Warn("create stdout log failed:", err)
		fout = ioutil.Discard
	} else {
		fout = p.OutputFile
	}
	cmd.Stdout = io.MultiWriter(p.Stdout, p.Output, fout)
	cmd.Stderr = io.MultiWriter(p.Stderr, p.Output, fout)
	// config environ
	cmd.Env = os.Environ() // inherit current vars
	environ := map[string]string{}
	if p.User != "" {
		if !IsRoot() {
			log.Warnf("detect not root, can not switch user")
		} else if err := cmd.SetUser(p.User); err != nil {
			log.Warnf("[%s] chusr to %s failed, %v", p.Name, p.User, err)
		} else {
			var homeDir string
			switch runtime.GOOS {
			case "linux":
				homeDir = "/home/" + p.User // FIXME(ssx): maybe there is a better way
			case "darwin":
				homeDir = "/Users/" + p.User
			}
			cmd.Env = append(cmd.Env, "HOME="+homeDir, "USER="+p.User)
			environ["HOME"] = homeDir
			environ["USER"] = p.User
		}
	}
	cmd.Env = append(cmd.Env, p.Environ...)
	mapping := func(key string) string {
		val := os.Getenv(key)
		if val != "" {
			return val
		}
		return environ[key]
	}
	cmd.Dir = os.Expand(p.Dir, mapping)
	if strings.HasPrefix(cmd.Dir, "~") {
		cmd.Dir = mapping("HOME") + cmd.Dir[1:]
	}
	log.Infof("[%s] use dir: %s\n", p.Name, cmd.Dir)
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
	p.mu.Lock()
	defer p.mu.Unlock()
	defer p.SetState(Stopped)
	if p.cmd == nil {
		return
	}

	p.SetState(Stopping)
	if p.cmd.Process != nil {
		p.cmd.Process.Signal(syscall.SIGTERM) // TODO(ssx): add it to config
	}
	select {
	case <-GoFunc(p.cmd.Wait):
		p.RunNotification(FSMState("quit normally"))
		log.Printf("program(%s) quit normally", p.Name)
	case <-time.After(time.Duration(p.StopTimeout) * time.Second): // TODO: add 3s to config
		p.RunNotification(FSMState("terminate all"))
		log.Printf("program(%s) terminate all", p.Name)
		p.cmd.Terminate(syscall.SIGKILL) // cleanup
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
}

func (p *Process) IsRunning() bool {
	return p.State() == Running || p.State() == RetryWait
}

func (p *Process) startCommand() {
	// p.Stdout.Reset()
	// p.Stderr.Reset()
	// p.Output.Reset() // Donot reset because log is still needed.
	log.Printf("start cmd(%s): %s", p.Name, p.Command)
	p.cmd = p.buildCommand()

	p.SetState(Running)
	if err := p.cmd.Start(); err != nil {
		log.Warnf("program %s start failed: %v", p.Name, err)
		p.SetState(Fatal)
		return
	}
	// 如果是running状态，重置 retryLeft
	p.retryLeft = p.StartRetries
	go func() {
		errC := GoFunc(p.cmd.Wait)
		startTime := time.Now()
		select {
		case <-errC:
			// if p.cmd.Wait() returns, it means program and its sub process all quited. no need to kill again
			// func Wait() will only return when program session finishs. (Only Tested on mac)
			log.Printf("program(%s) finished, time used %v", p.Name, time.Since(startTime))
			if time.Since(startTime) < time.Duration(p.StartSeconds)*time.Second {
				if p.retryLeft == p.StartRetries { // If first time quit so fast, just set to fatal
					p.SetState(Fatal)
					p.RunNotification(Fatal)
					log.Printf("program(%s) exit too quick, status -> fatal", p.Name)
					return
				}
			}
			p.waitNextRetry()
		case <-p.stopC:
			log.Println("recv stop command")
			p.stopCommand() // clean up all process
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
		// if newStatus == Fatal {
		if newStatus == RetryWait {
			pr.Program.retryCount++
		}
		go pr.Program.RunNotification(newStatus)
		// }
	}
	if pr.StartSeconds <= 0 {
		pr.StartSeconds = 3
	}
	if pr.StopTimeout <= 0 {
		pr.StopTimeout = 3
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
