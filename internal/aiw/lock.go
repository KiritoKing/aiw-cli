package aiw

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gofrs/flock"
)

func (r commandEnv) withLock(path string, action func() error) error {
	if err := r.ctx.Err(); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	lock := flock.New(path, flock.SetPermissions(0600))
	defer lock.Close()
	locked, err := lock.TryLock()
	if err != nil {
		return err
	}
	if !locked {
		fmt.Fprintf(r.ErrOut, "waiting for lock %s\n", path)
		locked, err = lock.TryLockContext(r.ctx, 100*time.Millisecond)
		if err != nil {
			return fmt.Errorf("wait for lock %s: %w", path, err)
		}
		if !locked {
			return fmt.Errorf("lock not acquired: %s", path)
		}
	}
	if err := r.ctx.Err(); err != nil {
		return err
	}
	return action()
}

func (c ChangeContext) withChangeLock(action func() error) error {
	return c.withLock(filepath.Join(c.Home, "locks", hash(c.Project), hash(c.Branch)+".lock"), action)
}
