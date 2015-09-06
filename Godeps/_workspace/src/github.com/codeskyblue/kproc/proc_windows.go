package kproc

import (
	"os"
	"os/exec"
	"strconv"
)

func ProcCommand(cmd *exec.Cmd) *Process {
	return &Process{
		Cmd: cmd,
	}
}

func ProcString(command string) *Process {
	cmd := exec.Command("cmd", "/c", command)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return &Process{
		Cmd: cmd,
	}
}

func (p *Process) Terminate(sig os.Signal) (err error) {
	if p.Process == nil {
		return nil
	}
	pid := p.Process.Pid
	c := exec.Command("taskkill", "/t", "/f", "/pid", strconv.Itoa(pid))
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}
