//go:build !unix

package indexer

import "os"

// Non-unix platforms do not expose syscall.Flock in the same way.
// Best-effort fallback: rely on single-process execution.
func tryLockFile(_ *os.File) error { return nil }

func unlockFile(_ *os.File) error { return nil }
