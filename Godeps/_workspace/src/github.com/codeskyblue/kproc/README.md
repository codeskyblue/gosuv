# kproc
[![GoDoc](https://godoc.org/github.com/codeskyblue/kproc?status.svg)](https://godoc.org/github.com/codeskyblue/kproc)

This is a golang lib, offer a better way to kill all child process.

Tested on _windows, linux, darwin._

This lib has been used in [fswatch](https://github.com/codeskyblue/fswatch).

## Usage

	go get -v github.com/codeskyblue/kproc

example:

	func main() {
		p := kproc.ProcString("python flask_main.py")
		p.Start()
		time.Sleep(3 * time.Second)
		err := p.Terminate(syscall.SIGKILL)
		if err != nil {
			log.Println(err)
		}
	}
