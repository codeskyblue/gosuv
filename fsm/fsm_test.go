package fsm

import (
	"log"
	"testing"
)

const (
	RUNNING = iota
	STOPPED
)

type RunningState int

func (s *RunningState) Enter() {
	log.Println("enter running state")
}

func (s *RunningState) Exit() {
	log.Println("out of running")
}

func (s *RunningState) CheckTransition(ev int) bool {
	return ev == RUNNING
}

type StoppedState int

func (s *StoppedState) Enter() {
	log.Println("Stopped enter")
}

func (s *StoppedState) Exit() {
	log.Println("Stopped exit")
}

func (s *StoppedState) CheckTransition(ev int) bool {
	return ev == STOPPED
}

func TestFSM(t *testing.T) {
	t.Log("Hello")
	rstate := new(RunningState)
	sstate := new(StoppedState)

	fsm := new(FSM)
	fsm.AddState("running", rstate)
	fsm.AddState("stopped", sstate)
	//fsm.SetDefaultState(sstate)
	fsm.Init()
	fsm.InputData(RUNNING)
	fsm.InputData(STOPPED)
	fsm.InputData(RUNNING)
}
