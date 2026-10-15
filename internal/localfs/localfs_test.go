package localfs

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestAtomicWriteWritesDataAndCleansTemporaryFiles(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "nested", "state.json")
	want := []byte("{\"ready\":true}\n")
	if err := AtomicWrite(path, want); err != nil {
		t.Fatal("AtomicWrite failed")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal("read AtomicWrite result")
	}
	if string(got) != string(want) {
		t.Fatal("AtomicWrite changed the file contents")
	}
	entries, err := os.ReadDir(filepath.Dir(path))
	if err != nil {
		t.Fatal("list AtomicWrite destination")
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatal("AtomicWrite left a temporary file behind")
	}
}

func TestAtomicWriteCleansTemporaryFileAfterRenameFailure(t *testing.T) {
	parent := t.TempDir()
	path := filepath.Join(parent, "existing-directory")
	if err := os.Mkdir(path, 0755); err != nil {
		t.Fatal("create rename target directory")
	}
	if err := AtomicWrite(path, []byte("data")); err == nil {
		t.Fatal("AtomicWrite unexpectedly replaced a directory")
	}
	entries, err := os.ReadDir(parent)
	if err != nil {
		t.Fatal("list AtomicWrite parent")
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(path) {
		t.Fatal("AtomicWrite left a temporary file after failure")
	}
}

func TestWithLockPropagatesActionErrorAndReleasesLock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "locks", "change.lock")
	callbackErr := errors.New("callback failed")
	if err := WithLock(path, func() error { return callbackErr }); err != callbackErr {
		t.Fatal("WithLock did not return the callback error")
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal("read lock file")
	}
	if info.Mode().Perm() != 0600 {
		t.Fatal("WithLock did not create a 0600 lock file")
	}

	finished := make(chan error, 1)
	go func() {
		finished <- WithLock(path, func() error { return nil })
	}()
	select {
	case err := <-finished:
		if err != nil {
			t.Fatal("second WithLock call failed")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("WithLock kept the lock after its callback failed")
	}
}
