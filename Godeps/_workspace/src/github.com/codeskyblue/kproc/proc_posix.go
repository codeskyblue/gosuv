// +build !windows

package kproc

import (
	"log"
	"os"
	"os/exec"
	"syscall"
)

func setupCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Setsid = true
}

func ProcCommand(cmd *exec.Cmd) *Process {
	setupCmd(cmd)
	return &Process{
		Cmd: cmd,
	}
}

func ProcString(command string) *Process {
	cmd := exec.Command("/bin/bash", "-c", command)
	setupCmd(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return &Process{
		Cmd: cmd,
	}
}

func (p *Process) Terminate(sig os.Signal) (err error) {
	if p.Process == nil {
		return
	}
	// find pgid, ref: http://unix.stackexchange.com/questions/14815/process-descendants
	group, err := os.FindProcess(-1 * p.Process.Pid)
	log.Println(group)
	if err == nil {
		err = group.Signal(sig)
	}
	return err
}
