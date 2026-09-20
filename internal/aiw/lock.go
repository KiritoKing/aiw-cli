package aiw

import (
	"os"
	"path/filepath"
	"syscall"
)

func withLock(path string, action func() error) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	if err := syscall.Flock(int(file.Fd()), syscall.LOCK_EX); err != nil {
		return err
	}
	defer syscall.Flock(int(file.Fd()), syscall.LOCK_UN)
	return action()
}

func changeLock(c ChangeContext, action func() error) error {
	return withLock(filepath.Join(c.Home, "locks", hash(c.Project), hash(c.Branch)+".lock"), action)
}
