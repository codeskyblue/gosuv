package main

import (
	"log"
	"syscall"
	"time"

	"github.com/codeskyblue/kexec"
)

func main() {
	p := kexec.CommandString("python flask_main.py")
	p.Start()
	time.Sleep(3 * time.Second)
	err := p.Terminate(syscall.SIGKILL)
	if err != nil {
		log.Println(err)
	}
}
