package log

import (
	"testing"
)

func TestFiles(t *testing.T) {
	w := NewFileWriter(FileOptions{ByType:ByDay})
	Std.SetOutput(w)
	Std.SetFlags(Std.Flags() | ^Llongcolor | ^Lshortcolor)
	Info("test")
	Debug("ssss")
	Warn("ssss")
	w.Close()
}
