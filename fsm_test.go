package main

import (
	"fmt"
	"os/exec"
	"strconv"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func findProcess(name string) bool {
	name = fmt.Sprintf("[%c]%s", name[0], name[1:])
	c := exec.Command("bash", "-c", fmt.Sprintf("ps -eo command | grep %s", strconv.Quote(name)))
	output, err := c.CombinedOutput()
	if err == nil {
		So(string(output), ShouldNotEqual, "")
		return true
	}
	return false
}

func TestStopCommand(t *testing.T) {
	Convey("Stop command should clean up all program", t, func() {
		p := NewProcess(Program{
			Name:        "sleep",
			Command:     "(echo hello; sleep 17&); exit 1",
			StopTimeout: 1,
		})
		p.startCommand()
		time.Sleep(100 * time.Millisecond)
		p.stopCommand()
		So(p.cmd, ShouldBeNil)
		exists := findProcess("sleep 17")
		So(exists, ShouldBeFalse)
	})
}
