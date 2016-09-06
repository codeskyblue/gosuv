package main

import (
	"sync"
	"testing"
	"time"

	. "github.com/smartystreets/goconvey/convey"
)

func TestBroadcast(t *testing.T) {
	bs := NewBroadcastString()
	bs.WriteMessage("hello")
	time.Sleep(10 * time.Millisecond)
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

func TestRingBuffer(t *testing.T) {
	Convey("Write some string to ring buffer", t, func() {
		// buf := rbuf.NewFixedSizeRingBuf(5)
		// buf.Write([]byte("abcde"))
		// So(string(buf.Bytes()), ShouldEqual, "abcde")
		// buf.Advance(2)
		// buf.Write([]byte("fg"))
		// So(string(buf.Bytes()), ShouldEqual, "cdefg")
	})
}
