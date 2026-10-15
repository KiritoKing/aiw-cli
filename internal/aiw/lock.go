package aiw

import (
	"path/filepath"

	"github.com/KiritoKing/aiw-cli/internal/localfs"
)

func changeLock(c ChangeContext, action func() error) error {
	return localfs.WithLock(filepath.Join(c.Home, "locks", hash(c.Project), hash(c.Branch)+".lock"), action)
}
