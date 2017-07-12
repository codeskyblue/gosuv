// +build linux darwin

package main

import (
	"os"
	"os/signal"
	"syscall"
)

func init() {
	go watchChildSignal()
}

func watchChildSignal() {
	var sigs = make(chan os.Signal, 3)
	signal.Notify(sigs, syscall.SIGCHLD)

	for {
		<-sigs
		reapChildren()
	}
}

func reapChildren() {
	var wstatus syscall.WaitStatus
	syscall.Wait4(-1, &wstatus, syscall.WNOHANG, nil)
}
