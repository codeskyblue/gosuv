package main

import (
	"sync"
	"time"

	"github.com/glycerine/rbuf"
	"github.com/qiniu/log"
)

type BroadcastString struct {
	msgC    chan string
	writers map[chan string]bool
	mu      sync.Mutex
}

func NewBroadcastString() *BroadcastString {
	b := &BroadcastString{
		msgC:    make(chan string, 20), // in case of cmd pipe error
		writers: make(map[chan string]bool, 0),
	}
	go func() {
		for message := range b.msgC {
			b.writeToAll(message)
		}
	}()
	return b
}

func (b *BroadcastString) writeToAll(message string) {
	for c := range b.writers {
		select {
		case c <- message:
		case <-time.After(500 * time.Millisecond):
			log.Println("channel closed, remove from queue")
			delete(b.writers, c)
		}
	}
}

func (b *BroadcastString) WriteMessage(message string) {
	select {
	case b.msgC <- message:
	default:
	}
}

func (b *BroadcastString) Reset() {
	b.msgC = make(chan string, 20)
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

func (b *BroadcastString) RemoveListener(c chan string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.writers, c)
}

func (b *BroadcastString) Close() {
	close(b.msgC)
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
	bufb := &BufferBroadcast{
		maxSize: size,
		bs:      NewBroadcastString(),
		buf:     rbuf.NewFixedSizeRingBuf(size), //  bytes.NewBuffer(nil), // buffer.NewRing(buffer.New(size)),
	}
	bufC := bufb.bs.AddListener(nil)
	go func() {
		for msg := range bufC {
			bufb.buf.Write([]byte(msg))
		}
	}()
	return bufb
}

func (b *BufferBroadcast) Write(data []byte) (n int, err error) {
	b.bs.WriteMessage(string(data)) // should return immediatiely, in case of pipe error
	return len(data), nil
}

func (b *BufferBroadcast) Reset() {
	b.buf.Reset()
	b.bs.Reset()
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
