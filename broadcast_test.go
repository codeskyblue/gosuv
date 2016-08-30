package main

import (
	"sync"
	"testing"
)

func TestBroadcast(t *testing.T) {
	bs := NewBroadcastString()
	bs.WriteMessage("hello")
	c1 := bs.AddListener(nil)
	go func() {
		bs.WriteMessage("world")
	}()
	message := <-c1
	if message != "world" {
		t.Fatalf("expect message world, but got %s", message)
	}
	c2 := bs.AddListener(nil)
	go func() {
		bs.WriteMessage("tab")
	}()

	// test write multi
	wg := sync.WaitGroup{}
	wg.Add(2)
	go func() {
		message = <-c2
		if message != "tab" {
			t.Errorf("expect tab, but got %s", message)
		}
		wg.Done()
	}()

	go func() {
		message = <-c1
		if message != "tab" {
			t.Errorf("expect tab, but got %s", message)
		}
		wg.Done()
	}()
	wg.Wait()
}
