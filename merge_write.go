package main

import (
	"bytes"
	"fmt"
	"io"
	"time"

	log "github.com/qiniu/log"
)

type Bool struct {
	c Int64
}

func (b *Bool) Get() bool {
	return b.c.Get() != 0
}

func (b *Bool) toInt64(v bool) int64 {
	if v {
		return 1
	} else {
		return 0
	}
}

func (b *Bool) Set(v bool) {
	b.c.Set(b.toInt64(v))
}

func (b *Bool) CompareAndSwap(o, n bool) bool {
	return b.c.CompareAndSwap(b.toInt64(o), b.toInt64(n))
}

func (b *Bool) Swap(v bool) bool {
	return b.c.Swap(b.toInt64(v)) != 0
}

var bufferPool *BufferPool

func init() {
	// 4000行日志缓存
	bufferPool = NewBufferPool(4000)
}

type MergeWriter struct {
	lines  chan *bytes.Buffer
	writer io.Writer
	closed Bool
}

func NewMergeWriter(writer io.Writer) *MergeWriter {
	merger := &MergeWriter{
		lines:  make(chan *bytes.Buffer, 1000),
		writer: writer,
	}
	merger.closed.Set(false)
	merger.drainLines()
	return merger
}

func (m *MergeWriter) Close() {
	// log.Printf("Close MergeWriter")
	if m.closed.CompareAndSwap(false, true) {
		// log.Printf("Close lines chan")
		close(m.lines)
	}
}

func (m *MergeWriter) WriteStrLine(line string) {
	if m.closed.Get() {
		return
	} else {
		buffer := bufferPool.Get()
		buffer.WriteString(line)
		m.lines <- buffer
	}
}

func (m *MergeWriter) WriteLine(line *bytes.Buffer) {
	if m.closed.Get() {
		// 需要回收Buffer
		// log.Printf("Write to closed MergeWrite...")
		bufferPool.Put(line)
		return
	} else {
		m.lines <- line
	}
}

func (m *MergeWriter) drainLines() {
	go func() {
		for line := range m.lines {
			m.writer.Write(line.Bytes())
			// 回收
			bufferPool.Put(line)
		}
	}()
}

// 创建新的BufferWriter
func (m *MergeWriter) NewWriter(index int) io.Writer {
	writer := &BufferWriter{
		merge:  m,
		prefix: fmt.Sprintf(" [P%02d] ", index),
	}

	// 分配
	writer.Buffer = bufferPool.Get()
	writer.Buffer.WriteString(writer.prefix)
	return writer
}

type BufferWriter struct {
	Buffer *bytes.Buffer
	prefix string
	merge  *MergeWriter
}

func (b *BufferWriter) Write(p []byte) (n int, err error) {
	n = len(p)

	for len(p) > 0 {
		index := bytes.IndexByte(p, '\n')
		if index != -1 {
			// 写完完整的一行
			_, err = b.Buffer.Write(p[0 : index+1])
			if err != nil {
				log.Error(err, "Writer Buffer failed")
				return n, err
			}

			// 将buffer转移到merge中
			b.merge.WriteLine(b.Buffer)

			// 分配：写入新数据
			b.Buffer = bufferPool.Get()
			b.Buffer.WriteString(time.Now().Format("15:04:05") + b.prefix)
			p = p[index+1:]
		} else {
			// 剩下不足一行，一口气全部写入
			_, err = b.Buffer.Write(p)
			if err != nil {
				log.Error(err, "Writer Buffer failed")
				return n, err
			}
			break
		}
	}
	return n, nil
}
