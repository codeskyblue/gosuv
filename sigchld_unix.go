// +build linux darwin

package main

import (
	"log"
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
	for {
		var wstatus syscall.WaitStatus
		wpid, err := syscall.Wait4(-1, &wstatus, syscall.WNOHANG, nil)
		if err != nil {
			log.Printf("syscall.Wait4 call failed: %v", err)
			break
		}

		if wpid != 0 {
			log.Printf("reap dead child: %d, wstatus: %#08x", wpid, wstatus)
		} else {
			break
		}
	}
}
