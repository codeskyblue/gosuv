package main

import (
	"errors"
	"io"
	"sync"
	"time"

	"github.com/codeskyblue/rbuf"
	"github.com/qiniu/log"
)

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

func (wb *WriteBroadcaster) Closed() bool {
	return wb.closed
}

// this is main func
func (wb *WriteBroadcaster) NewChanString(name string) chan string {
	wb.Lock()
	defer wb.Unlock()

	wr := NewChanStrWriter()
	if wb.closed {
		wr.Close()
		return nil
	}
	sw := StreamWriter{wc: wr, stream: name}
	wb.writers[sw] = true
	wr.Write(wb.buf.Bytes())
	return wr.C
}

func (wb *WriteBroadcaster) Bytes() []byte {
	return wb.buf.Bytes()
}

func (w *WriteBroadcaster) Write(p []byte) (n int, err error) {
	w.Lock()
	defer w.Unlock()

	// write with advance
	w.buf.WriteAndMaybeOverwriteOldestData(p)

	for sw := range w.writers {
		// set write timeout
		err = GoTimeout(func() error {
			if _, err := sw.wc.Write(p); err != nil { //|| n != len(p) {
				return errors.New("broadcast to " + sw.stream + " error")
			}
			return nil
		}, time.Second*1)
		if err != nil {
			// On error, evict the writer
			log.Warnf("broadcase write error: %s, %s", sw.stream, err)
			sw.wc.Close()
			delete(w.writers, sw)
		}
	}
	return len(p), nil
}

func (w *WriteBroadcaster) CloseWriter(name string) {
	for sw := range w.writers {
		if sw.stream == name {
			sw.wc.Close()
		}
	}
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

// chan string writer
type chanStrWriter struct {
	C      chan string
	closed bool
}

func (c *chanStrWriter) Write(data []byte) (n int, err error) {
	if c.closed {
		return 0, errors.New("chan writer closed")
	}
	c.C <- string(data) // write timeout
	return len(data), nil
}

func (c *chanStrWriter) Close() error {
	if !c.closed {
		c.closed = true
		close(c.C)
	}
	return nil
}

func NewChanStrWriter() *chanStrWriter {
	return &chanStrWriter{
		C: make(chan string, 10),
	}
}

// quick loss writer
type QuickLossBroadcastWriter struct {
	*WriteBroadcaster
	bufC   chan string
	closed bool
}

func (w *QuickLossBroadcastWriter) Write(buf []byte) (int, error) {
	select {
	case w.bufC <- string(buf):
	default:
	}
	return len(buf), nil
}

func (w *QuickLossBroadcastWriter) Close() error {
	if !w.closed {
		w.closed = true
		close(w.bufC)
		w.WriteBroadcaster.CloseWriters()
	}
	return nil
}

func (w *QuickLossBroadcastWriter) drain() {
	for data := range w.bufC {
		w.WriteBroadcaster.Write([]byte(data))
	}
}

func NewQuickLossBroadcastWriter(size int) *QuickLossBroadcastWriter {
	qlw := &QuickLossBroadcastWriter{
		WriteBroadcaster: NewWriteBroadcaster(size),
		bufC:             make(chan string, 20),
	}
	go qlw.drain()
	return qlw
}
