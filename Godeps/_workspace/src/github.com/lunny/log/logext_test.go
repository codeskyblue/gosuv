package log

import (
	"database/sql"
	"fmt"
	"testing"

	_ "github.com/mattn/go-sqlite3"
)

func TestLog(t *testing.T) {

	SetOutputLevel(Ldebug)

	Debugf("Debug: foo\n")
	Debug("Debug: foo")

	Infof("Info: foo\n")
	Info("Info: foo")

	Warnf("Warn: foo\n")
	Warn("Warn: foo")

	Errorf("Error: foo\n")
	Error("Error: foo")

	SetOutputLevel(Linfo)

	Debugf("Debug: foo\n")
	Debug("Debug: foo")

	Infof("Info: foo\n")
	Info("Info: foo")

	Warnf("Warn: foo\n")
	Warn("Warn: foo")

	Errorf("Error: foo\n")
	Error("Error: foo")

	db, err := sql.Open("sqlite3", "./log.db")
	if err != nil {
		t.Fatal(err)
	}

	w, err := NewDBWriter(db)
	if err != nil {
		fmt.Println(err)
		t.Fatal(err)
	}
	SetOutput(w)
	SetFlags(RmColorFlags(Flags()))
	// output all
	SetOutputLevel(Lall)
	Debugf("Debug: foo\n")
	Debug("Debug: foo")

	Infof("Info: foo\n")
	Info("Info: foo")

	Warnf("Warn: foo\n")
	Warn("Warn: foo")

	Errorf("Error: foo\n")
	Error("Error: foo")

	// output none
	SetOutputLevel(Lnone)
	Warnf("Warn: foo\n")
	Warn("Warn: foo")

	Errorf("Error: foo\n")
	Error("Error: foo")

}
