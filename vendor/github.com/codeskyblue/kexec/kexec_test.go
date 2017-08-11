package kexec

import (
	"os"
	"os/user"
	"syscall"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestCommand(t *testing.T) {
	Convey("1 should equal 1", t, func() {
		So(1, ShouldEqual, 1)
	})

	Convey("kexec should work as normal os/exec", t, func() {
		cmd := Command("echo", "-n", "123")
		data, err := cmd.Output()
		So(err, ShouldBeNil)
		So(string(data), ShouldEqual, "123")
	})

	Convey("the terminate should kill proc", t, func() {
		cmd := CommandString("sleep 51")
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		cmd.Start()
		time.Sleep(time.Millisecond * 50)
		cmd.Terminate(syscall.SIGINT)
		err := cmd.Wait()
		So(err, ShouldNotBeNil)
		//So(err.Error(), ShouldEqual, "signal: interrupt")
	})

	Convey("Should ok with call Wait twice", t, func() {
		cmd := CommandString("not-exists-command-xxl213 true")
		var err error
		err = cmd.Start()
		So(err, ShouldBeNil)

		err1 := cmd.Wait()
		So(err1, ShouldNotBeNil)
		err2 := cmd.Wait()
		So(err1, ShouldEqual, err2)
	})

	Convey("Set user works", t, func() {
		u, err := user.Current()
		So(err, ShouldBeNil)
		// Set user must be root
		if u.Uid != "0" {
			return
		}

		cmd := Command("whoami")
		err = cmd.SetUser("qard2")
		So(err, ShouldBeNil)

		output, err := cmd.Output()
		So(err, ShouldBeNil)
		So(string(output), ShouldEqual, "qard2\n")
	})
}
