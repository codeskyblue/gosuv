# kexec
[![GoDoc](https://godoc.org/github.com/codeskyblue/kexec?status.svg)](https://godoc.org/github.com/codeskyblue/kexec)

This is a golang lib, add a `Terminate` command to exec.

Tested on _windows, linux, darwin._

This lib has been used in [fswatch](https://github.com/codeskyblue/fswatch).

## Usage

	go get -v github.com/codeskyblue/kexec


example1:

	package main

	import "github.com/codeskyblue/kexec"

	func main(){
		p := kexec.Command("python", "flask_main.py")
		p.Start()
		p.Terminate(syscall.SIGINT)
	}
	
example2: see more [examples](examples)

	package main
	
	import "github.com/codeskyblue/kexec"

	func main() {
		// In unix will call: bash -c "python flask_main.py"
		// In windows will call: cmd /c "python flask_main.py"
		p := kexec.CommandString("python flask_main.py")
		p.Start()
		p.Terminate(syscall.SIGKILL)
	}

## PS
This lib also support you call `Wait()` twice, which is not support by `os/exec`

## LICENSE
[MIT](LICENSE)