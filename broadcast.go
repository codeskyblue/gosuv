package main

import (
	"errors"
	"io"
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
		c = make(chan string, 4)
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

// The new broadcast
type StreamWriter struct {
	wc     io.WriteCloser
	stream string
}

type WriteBroadcaster struct {
	sync.Mutex
	buf     *rbuf.FixedSizeRingBuf
	writers map[StreamWriter]bool
	closed  bool
}

func NewWriteBroadcaster(size int) *WriteBroadcaster {
	if size <= 0 {
		size = 4 * 1024
	}
	bc := &WriteBroadcaster{
		writers: make(map[StreamWriter]bool),
		buf:     rbuf.NewFixedSizeRingBuf(size),
		closed:  false,
	}
	return bc
}

func (w *WriteBroadcaster) AddWriter(writer io.WriteCloser, stream string) {
	w.Lock()
	defer w.Unlock()
	if w.closed {
		writer.Close()
		return
	}
	sw := StreamWriter{wc: writer, stream: stream}
	w.writers[sw] = true
}

func (wb *WriteBroadcaster) Closed() bool {
	return wb.closed
}

func (wb *WriteBroadcaster) NewReader(name string) ([]byte, *io.PipeReader) {
	r, w := io.Pipe()
	wb.AddWriter(w, name)
	return wb.buf.Bytes(), r
}

func (wb *WriteBroadcaster) Bytes() []byte {
	return wb.buf.Bytes()
}

func (w *WriteBroadcaster) Write(p []byte) (n int, err error) {
	w.Lock()
	defer w.Unlock()
	w.buf.Write(p)
	for sw := range w.writers {
		// set write timeout
		err = GoTimeout(func() error {
			if n, err := sw.wc.Write(p); err != nil || n != len(p) {
				return errors.New("broadcast to " + sw.stream + " error")
			}
			return nil
		}, time.Second*1)
		if err != nil {
			// On error, evict the writer
			log.Warnf("broadcase write error: %s, %s", sw.stream, err)
			delete(w.writers, sw)
		}
	}
	return len(p), nil
}

func (w *WriteBroadcaster) CloseWriters() error {
	w.Lock()
	defer w.Unlock()
	for sw := range w.writers {
		sw.wc.Close()
	}
	w.writers = make(map[StreamWriter]bool)
	w.closed = true
	return nil
}

// nop writer
type NopWriter struct{}

func (*NopWriter) Write(buf []byte) (int, error) {
	return len(buf), nil
}

type nopWriteCloser struct {
	io.Writer
}

func (w *nopWriteCloser) Close() error { return nil }

func NopWriteCloser(w io.Writer) io.WriteCloser {
	return &nopWriteCloser{w}
}
