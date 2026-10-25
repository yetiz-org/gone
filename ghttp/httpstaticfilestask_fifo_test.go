//go:build unix

package ghttp

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func TestStaticFilesSecurity_FIFODoesNotHoldLoadSlot(t *testing.T) {
	root := t.TempDir()
	if err := syscall.Mkfifo(filepath.Join(root, "pipe.txt"), 0600); err != nil {
		t.Skip("mkfifo unsupported:", err)
	}
	if err := os.WriteFile(filepath.Join(root, "good.txt"), []byte("ok"), 0600); err != nil {
		t.Fatal(err)
	}
	for _, legacy := range []bool{false, true} {
		handler := NewStaticFilesHandlerTask(root)
		handler.DoMinify, handler.MaxConcurrentLoads, handler.AllowExternalSymlinks = false, 1, legacy
		if resp := staticSecurityRequest(t, handler, "/pipe.txt"); resp.StatusCode() < 400 {
			t.Fatalf("legacy=%v: FIFO must be rejected: status=%d", legacy, resp.StatusCode())
		}
		if resp := staticSecurityRequest(t, handler, "/good.txt"); resp.StatusCode() != 200 || string(resp.Body().Bytes()) != "ok" {
			t.Fatalf("legacy=%v: FIFO request must not hold the load slot: status=%d", legacy, resp.StatusCode())
		}
	}
}
