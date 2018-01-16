package gops

import (
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"

	mps "github.com/mitchellh/go-ps"
)

type Process struct {
	mps.Process
}

func NewProcess(pid int) (p Process, err error) {
	mp, err := mps.FindProcess(pid)
	if err != nil {
		return
	}
	return Process{
		Process: mp,
	}, nil
}

// func (p *Process) Mem() (m sigar.ProcMem, err error) {
// 	err = m.Get(p.Pid())
// 	return
// }

type ProcInfo struct {
	Pid  int     `json:"pid"`
	Pids []int   `json:"pids"`
	Rss  int     `json:"rss"`
	PCpu float64 `json:"pcpu"`
}

func (pi *ProcInfo) Add(add ProcInfo) {
	pi.Rss += add.Rss
	pi.PCpu += add.PCpu
}

// CPU Percent * 100
// only linux and darwin works
func (p *Process) ProcInfo() (pi ProcInfo, err error) {
	pi.Pid = p.Pid()
	cmd := exec.Command("ps", "-o", "pcpu,rss", "-p", strconv.Itoa(p.Pid()))
	output, err := cmd.Output()
	if err != nil {
		err = errors.New("ps err: " + err.Error())
		return
	}

	fields := strings.SplitN(string(output), "\n", 2)
	if len(fields) != 2 {
		err = errors.New("parse ps command out format error")
		return
	}
	_, err = fmt.Sscanf(fields[1], "%f %d", &pi.PCpu, &pi.Rss)
	pi.Rss *= 1024
	return
}

// Get all child process
func (p *Process) Children(recursive bool) (cps []Process) {
	pses, err := mps.Processes()
	if err != nil {
		return
	}
	pidMap := make(map[int][]mps.Process, 0)
	for _, p := range pses {
		pidMap[p.PPid()] = append(pidMap[p.PPid()], p)
	}
	var travel func(int)
	travel = func(pid int) {
		for _, p := range pidMap[pid] {
			cps = append(cps, Process{p})
			if recursive {
				travel(p.Pid())
			}
		}
	}
	travel(p.Pid())
	return
}

//Sum everything
func (p *Process) ChildrenProcInfo(recursive bool) (pi ProcInfo) {
	cps := p.Children(recursive)
	for _, cp := range cps {
		info, er := cp.ProcInfo()
		if er != nil {
			continue
		}
		pi.Add(info)
		pi.Pids = append(pi.Pids, cp.Pid())
	}
	pi.Pid = p.Pid()
	return
}
