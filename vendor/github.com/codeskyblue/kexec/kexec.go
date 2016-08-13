package kexec

import "os/exec"

type KCommand struct {
	*exec.Cmd
}
