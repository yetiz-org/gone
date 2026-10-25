package ghttp

import (
	"bytes"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/tdewolff/minify/v2"
)

func TestStaticFilesSecurity_RootContainment(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "inside.txt"), []byte("public"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("secret"), 0600); err != nil {
		t.Fatal(err)
	}
	for name, target := range map[string]string{
		"internal.txt": "inside.txt",
		"external.txt": filepath.Join(outside, "secret.txt"),
		"external-dir": outside,
	} {
		if err := os.Symlink(target, filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewStaticFilesHandlerTask(root)
	handler.DoMinify, handler.DoCache = false, false
	for _, scenario := range []struct {
		path    string
		allowed bool
	}{
		{"inside.txt", true},
		{"internal.txt", true},
		{"external.txt", false},
		{"external-dir/secret.txt", false},
	} {
		t.Run(scenario.path, func(t *testing.T) {
			req := &Request{request: httptest.NewRequest("GET", "http://gone.test/"+scenario.path, nil)}
			resp := NewResponse(req)
			if err := handler.Get(nil, req, resp, nil); err != nil {
				t.Fatal(err)
			}
			if scenario.allowed {
				if resp.StatusCode() != 200 || resp.Body() == nil || !bytes.Equal(resp.Body().Bytes(), []byte("public")) {
					t.Fatalf("normal asset response: status=%d body=%v", resp.StatusCode(), resp.Body())
				}
			} else if resp.StatusCode() < 400 || resp.Body() != nil && bytes.Contains(resp.Body().Bytes(), []byte("secret")) {
				t.Fatalf("outside-root asset served: status=%d", resp.StatusCode())
			}
		})
	}
	legacy := NewStaticFilesHandlerTask(root)
	legacy.DoMinify, legacy.AllowExternalSymlinks = false, true
	if resp := staticSecurityRequest(t, legacy, "/external.txt"); resp.StatusCode() != 200 || string(resp.Body().Bytes()) != "secret" {
		t.Fatalf("explicit legacy symlink opt-in failed: status=%d", resp.StatusCode())
	}
}

func TestStaticFilesSecurity_DefaultFileLimit(t *testing.T) {
	root := t.TempDir()
	file, err := os.Create(filepath.Join(root, "oversized.bin"))
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate((16 << 20) + 1); err != nil {
		file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	handler := NewStaticFilesHandlerTask(root)
	req := &Request{request: httptest.NewRequest("GET", "http://gone.test/oversized.bin", nil)}
	resp := NewResponse(req)
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	if err := handler.Get(nil, req, resp, nil); err != nil {
		t.Fatal(err)
	}
	runtime.ReadMemStats(&after)
	if resp.StatusCode() != 503 || resp.Body() != nil && resp.Body().ReadableBytes() != 0 {
		t.Errorf("oversized file must fail before response allocation: status=%d", resp.StatusCode())
	}
	if allocated := after.TotalAlloc - before.TotalAlloc; allocated >= 8<<20 {
		t.Errorf("oversized rejection allocated %d bytes; full file must not be read", allocated)
	}
}

func TestStaticFilesSecurity_ConfiguredBounds(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"boundary.txt": "12345678", "oversized.txt": "123456789", "expand.txt": "expand", "good.txt": "ok"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewStaticFilesHandlerTask(root)
	handler.DoMinify, handler.DoCache, handler.MaxFileBytes = false, false, 8
	if resp := staticSecurityRequest(t, handler, "/boundary.txt"); resp.StatusCode() != 200 || string(resp.Body().Bytes()) != "12345678" || !strings.HasPrefix(resp.GetHeader("Content-Type"), "text/plain") {
		t.Fatalf("exact file limit must preserve content and MIME: status=%d", resp.StatusCode())
	}
	if resp := staticSecurityRequest(t, handler, "/oversized.txt"); resp.StatusCode() != 503 || resp.Body().ReadableBytes() != 0 {
		t.Fatalf("file limit+1 must reject without content: status=%d", resp.StatusCode())
	}
	minifying := NewStaticFilesHandlerTask(root)
	minifying.MaxFileBytes, minifying.MaxConcurrentLoads = 8, 1
	minifying.m.AddFunc("text/plain", func(_ *minify.M, w io.Writer, r io.Reader, _ map[string]string) error {
		data, err := io.ReadAll(r)
		if err != nil {
			return err
		}
		if string(data) == "expand" {
			_, _ = io.WriteString(w, "123456789")
			_, err = w.Write(nil)
		} else {
			_, err = w.Write(data)
		}
		return err
	})
	if resp := staticSecurityRequest(t, minifying, "/expand.txt"); resp.StatusCode() != 503 || resp.Body().ReadableBytes() != 0 {
		t.Errorf("minifier output growth must reject without content: status=%d bytes=%d", resp.StatusCode(), resp.Body().ReadableBytes())
	}
	if resp := staticSecurityRequest(t, minifying, "/good.txt"); resp.StatusCode() != 200 || string(resp.Body().Bytes()) != "ok" {
		t.Fatalf("failed load must release slot: status=%d", resp.StatusCode())
	}
	if resp := staticSecurityRequest(t, minifying, "/missing.txt"); resp.StatusCode() != 404 {
		t.Fatalf("missing file status=%d, want 404", resp.StatusCode())
	}
}

func TestStaticFilesSecurity_CacheEviction(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"a.txt": "aaaa", "b.txt": "bbbb", "c.txt": "cccc", "empty-a.txt": "", "empty-b.txt": ""} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewStaticFilesHandlerTask(root)
	handler.DoMinify, handler.MaxCacheBytes, handler.MaxCacheEntries = false, 8, 3
	for _, name := range []string{"a.txt", "b.txt"} {
		if resp := staticSecurityRequest(t, handler, "/"+name); resp.StatusCode() != 200 {
			t.Fatal(resp.StatusCode())
		}
		if err := os.WriteFile(filepath.Join(root, name), []byte(strings.Repeat(strings.ToUpper(name[:1]), 4)), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if resp := staticSecurityRequest(t, handler, "/a.txt"); string(resp.Body().Bytes()) != "aaaa" {
		t.Fatal("a must be cached before LRU eviction")
	}
	staticSecurityRequest(t, handler, "/c.txt")
	if resp := staticSecurityRequest(t, handler, "/a.txt"); string(resp.Body().Bytes()) != "aaaa" {
		t.Fatal("recently used a must survive byte-budget eviction")
	}
	if resp := staticSecurityRequest(t, handler, "/b.txt"); string(resp.Body().Bytes()) != "BBBB" {
		t.Fatal("least-recently-used b must reload after byte-budget eviction")
	}
	entries := NewStaticFilesHandlerTask(root)
	entries.DoMinify, entries.MaxCacheEntries = false, 1
	staticSecurityRequest(t, entries, "/empty-a.txt")
	if err := os.WriteFile(filepath.Join(root, "empty-a.txt"), []byte("new"), 0600); err != nil {
		t.Fatal(err)
	}
	staticSecurityRequest(t, entries, "/empty-b.txt")
	if resp := staticSecurityRequest(t, entries, "/empty-a.txt"); string(resp.Body().Bytes()) != "new" {
		t.Fatal("empty files must consume cache entry budget")
	}
	uncached := NewStaticFilesHandlerTask(root)
	uncached.DoCache, uncached.DoMinify = false, false
	staticSecurityRequest(t, uncached, "/empty-a.txt")
	if err := os.WriteFile(filepath.Join(root, "empty-a.txt"), []byte("fresh"), 0600); err != nil {
		t.Fatal(err)
	}
	if resp := staticSecurityRequest(t, uncached, "/empty-a.txt"); string(resp.Body().Bytes()) != "fresh" {
		t.Fatal("DoCache=false must reload file content")
	}
}

func TestStaticFilesSecurity_ConcurrentLoads(t *testing.T) {
	root := t.TempDir()
	for name, content := range map[string]string{"first.txt": "first", "second.txt": "second"} {
		if err := os.WriteFile(filepath.Join(root, name), []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	handler := NewStaticFilesHandlerTask(root)
	handler.MaxConcurrentLoads = 1
	entered, release := make(chan struct{}, 1), make(chan struct{})
	releaseLoad := sync.OnceFunc(func() { close(release) })
	t.Cleanup(releaseLoad)
	var calls atomic.Int32
	ctx := t.Context()
	handler.m.AddFunc("text/plain", func(_ *minify.M, w io.Writer, r io.Reader, _ map[string]string) error {
		calls.Add(1)
		select {
		case entered <- struct{}{}:
		default:
		}
		select {
		case <-release:
		case <-ctx.Done():
			return ctx.Err()
		}
		_, err := io.Copy(w, r)
		return err
	})
	responses := make(chan *Response, 17)
	var workers sync.WaitGroup
	workers.Go(func() { responses <- staticSecurityRequest(t, handler, "/first.txt") })
	defer func() {
		releaseLoad()
		workers.Wait()
	}()
	select {
	case <-entered:
	case <-time.After(2 * time.Second):
		t.Fatal("first load did not enter minifier")
	}
	started := make(chan struct{}, 16)
	for range 16 {
		workers.Go(func() {
			started <- struct{}{}
			responses <- staticSecurityRequest(t, handler, "/first.txt")
		})
	}
	for range 16 {
		<-started
	}
	if resp := staticSecurityRequest(t, handler, "/second.txt"); resp.StatusCode() != 503 {
		t.Errorf("distinct-path saturated load status=%d, want 503", resp.StatusCode())
	}
	remaining := 17
	select {
	case resp := <-responses:
		remaining--
		t.Errorf("same-path load returned before shared work completed: status=%d", resp.StatusCode())
	case <-time.After(50 * time.Millisecond):
	}
	releaseLoad()
	for range remaining {
		select {
		case resp := <-responses:
			if resp.StatusCode() != 200 || string(resp.Body().Bytes()) != "first" {
				t.Errorf("coalesced load result status=%d", resp.StatusCode())
			}
		case <-time.After(2 * time.Second):
			t.Fatal("coalesced load did not terminate")
		}
	}
	if calls.Load() != 1 {
		t.Errorf("same-path loads performed %d minifications, want 1", calls.Load())
	}
	if resp := staticSecurityRequest(t, handler, "/second.txt"); resp.StatusCode() != 200 || string(resp.Body().Bytes()) != "second" {
		t.Errorf("completed load must return slot: status=%d", resp.StatusCode())
	}
}

func TestStaticFilesSecurity_SymlinkReplacement(t *testing.T) {
	root, outside := t.TempDir(), t.TempDir()
	if err := os.Mkdir(filepath.Join(root, "inside"), 0700); err != nil {
		t.Fatal(err)
	}
	for filePath, content := range map[string]string{filepath.Join(root, "inside", "asset.txt"): "public", filepath.Join(outside, "asset.txt"): "secret"} {
		if err := os.WriteFile(filePath, []byte(content), 0600); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.Symlink("inside", filepath.Join(root, "pivot")); err != nil {
		t.Fatal(err)
	}
	handler := NewStaticFilesHandlerTask(root)
	handler.DoCache, handler.DoMinify = false, false
	errors := make(chan error, 1)
	var writer sync.WaitGroup
	writer.Go(func() {
		for i := range 200 {
			target := "inside"
			if i%2 != 0 {
				target = outside
			}
			if err := os.Symlink(target, filepath.Join(root, "next")); err != nil {
				errors <- err
				return
			}
			if err := os.Rename(filepath.Join(root, "next"), filepath.Join(root, "pivot")); err != nil {
				errors <- err
				return
			}
		}
	})
	defer writer.Wait()
	for range 200 {
		resp := staticSecurityRequest(t, handler, "/pivot/asset.txt")
		if resp.StatusCode() == 200 && string(resp.Body().Bytes()) != "public" {
			t.Fatal("symlink replacement served outside-root bytes")
		}
	}
	writer.Wait()
	select {
	case err := <-errors:
		t.Fatal(err)
	default:
	}
}

func staticSecurityRequest(t *testing.T, handler *StaticFilesHandlerTask, uri string) (resp *Response) {
	t.Helper()
	req := &Request{request: httptest.NewRequest("GET", "http://gone.test"+uri, nil)}
	resp = NewResponse(req)
	if err := handler.Get(nil, req, resp, nil); err != nil {
		t.Fatal(err)
	}
	return resp
}
