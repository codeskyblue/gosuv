package main

import (
	"log"
	"sync"
	"time"

	"github.com/glycerine/rbuf"
)

type BroadcastString struct {
	writers map[chan string]bool
	mu      sync.Mutex
}

func NewBroadcastString() *BroadcastString {
	return &BroadcastString{
		writers: make(map[chan string]bool, 0),
	}
}

func (b *BroadcastString) WriteMessage(message string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for c := range b.writers {
		select {
		case c <- message:
		case <-time.After(500 * time.Millisecond):
			log.Println("channel closed, remove from queue")
			delete(b.writers, c)
		}
	}
}

func (b *BroadcastString) AddListener(c chan string) chan string {
	b.mu.Lock()
	defer b.mu.Unlock()
	if c == nil {
		c = make(chan string, 0)
	}
	b.writers[c] = true
	return c
}

type BufferBroadcast struct {
	bs *BroadcastString

	maxSize int
	buf     *rbuf.FixedSizeRingBuf // *bytes.Buffer
	mu      sync.Mutex
}

func NewBufferBroadcast(size int) *BufferBroadcast {
	if size <= 0 {
		size = 4 * 1024 // 4K
	}
	return &BufferBroadcast{
		maxSize: size,
		bs:      NewBroadcastString(),
		buf:     rbuf.NewFixedSizeRingBuf(size), //  bytes.NewBuffer(nil), // buffer.NewRing(buffer.New(size)),
	}
}

func (b *BufferBroadcast) Write(data []byte) (n int, err error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	// if b.buf.Len() >= b.maxSize*2 {
	// 	b.buf = bytes.NewBuffer(b.buf.Bytes()[b.buf.Len()-b.maxSize : b.buf.Len()])
	// }
	b.bs.WriteMessage(string(data))
	return b.buf.Write(data)
}

func (b *BufferBroadcast) Reset() {
	b.buf.Reset()
}

func (b *BufferBroadcast) AddHookFunc(wf func(string) error) chan error {
	b.mu.Lock()
	defer b.mu.Unlock()
	c := b.bs.AddListener(nil)
	errC := make(chan error, 1)
	go func() {
		data := b.buf.Bytes()
		// data, _ := ioutil.ReadAll(b.buf)
		if err := wf(string(data)); err != nil {
			errC <- err
			return
		}
		for msg := range c {
			err := wf(msg)
			if err != nil {
				errC <- err
				break
			}
		}
	}()
	return errC
}
