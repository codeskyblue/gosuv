package main

import (
	"fmt"
	"kproc"
	"log"
	"os/exec"
	"syscall"
	"time"
)

func main() {
	p := kproc.ProcString("python flask_main.py")
	p.Start()
	time.Sleep(10 * time.Second)
	err := p.Terminate(syscall.SIGKILL)
	if err != nil {
		log.Println(err)
	}
	out, _ := exec.Command("lsof", "-i:5000").CombinedOutput()
	fmt.Println(string(out))
}
