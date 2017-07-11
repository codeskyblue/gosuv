// +build linux darwin

package main

import (
	"syscall"

	"github.com/qiniu/log"
)

func init() {
	go reapChildren()
}

func reapChildren() {
	var wstatus syscall.WaitStatus

	for {
		pid, err := syscall.Wait4(-1, &wstatus, 0, nil)
		for err == syscall.EINTR {
			pid, err = syscall.Wait4(-1, &wstatus, 0, nil)
		}
		if err == syscall.ECHILD {
			break
		}
		log.Printf("pid %d, finished, wstatus: %+v", pid, wstatus)
	}
}
