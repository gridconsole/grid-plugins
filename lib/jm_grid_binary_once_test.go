package lib

import (
	"fmt"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

// The grid binary is built once per package run, into a folder that TestMain owns.
var (
	jmGridDir    string
	jmGridOnce   sync.Once
	jmGridBinary string
	jmGridErr    error
	jmGridBuilds atomic.Int32
)

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "jm-grid-")
	if err != nil {
		fmt.Fprintf(os.Stderr, "jm: %v\n", err)
		os.Exit(1)
	}
	jmGridDir = dir
	code := m.Run()
	if n := jmGridBuilds.Load(); n > 1 {
		fmt.Fprintf(os.Stderr, "jm: grid built %d times\n", n)
		code = 1
	}
	os.RemoveAll(dir)
	os.Exit(code)
}

func TestJmGridBinaryBuiltOnce(t *testing.T) {
	first := buildBinary(t)
	second := buildBinary(t)
	if first != second {
		t.Fatalf("paths differ: %s %s", first, second)
	}
	if _, err := os.Stat(first); err != nil {
		t.Fatal(err)
	}
	if n := jmGridBuilds.Load(); n != 1 {
		t.Fatalf("grid built %d times", n)
	}
}
