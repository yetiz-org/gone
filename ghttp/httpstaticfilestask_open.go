//go:build !wasm

package ghttp

import (
	"os"
	"syscall"
)

// O_NONBLOCK keeps a FIFO from blocking the open and holding a load slot; the regular-file check rejects it.
const staticOpenFlags = os.O_RDONLY | syscall.O_NONBLOCK
