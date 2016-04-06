// +build !windows

package kexec

import (
	"os"
	"os/exec"
	"syscall"
)

func setupCmd(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{}
	cmd.SysProcAttr.Setsid = true
}

func Command(name string, arg ...string) *KCommand {
	cmd := exec.Command(name, arg...)
	setupCmd(cmd)
	return &KCommand{
		Cmd: cmd,
	}
}

func CommandString(command string) *KCommand {
	cmd := exec.Command("/bin/bash", "-c", command)
	setupCmd(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return &KCommand{
		Cmd: cmd,
	}
}

func (p *KCommand) Terminate(sig os.Signal) (err error) {
	if p.Process == nil {
		return
	}
	// find pgid, ref: http://unix.stackexchange.com/questions/14815/process-descendants
	group, err := os.FindProcess(-1 * p.Process.Pid)
	//log.Println(group)
	if err == nil {
		err = group.Signal(sig)
	}
	return err
}
