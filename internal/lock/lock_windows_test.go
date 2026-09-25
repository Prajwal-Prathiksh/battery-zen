//go:build windows

package lock

import (
	"path/filepath"
	"testing"
)

func TestNamedMutexExcludesSecondInstance(t *testing.T) {
	path := filepath.Join(t.TempDir(), "battery-zen.lock")
	first := New(path)
	second := New(path)
	acquired, err := first.Acquire()
	if err != nil || !acquired {
		t.Fatalf("first acquire = %t, %v", acquired, err)
	}
	defer first.Release()
	acquired, err = second.Acquire()
	if err != nil {
		t.Fatal(err)
	}
	if acquired {
		second.Release()
		t.Fatal("second instance acquired the same mutex")
	}
}
