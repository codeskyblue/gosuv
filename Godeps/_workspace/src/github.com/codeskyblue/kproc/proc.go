package kproc

import "os/exec"

type Process struct {
	*exec.Cmd
}
