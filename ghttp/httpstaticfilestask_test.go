package ghttp

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yetiz-org/gone/channel"
	buf "github.com/yetiz-org/goth-bytebuf"
	kklogger "github.com/yetiz-org/goth-kklogger"
)

// TestParseRange_SpecifiedRange tests parsing of "bytes=start-end" format
func TestParseRange_SpecifiedRange(t *testing.T) {
	handler := NewStaticFilesHandlerTask("")
	fileSize := int64(1000)

	testCases := []struct {
		name        string
		rangeHeader string
		fileSize    int64
		wantStart   int64
		wantEnd     int64
		wantValid   bool
	}{
		{"valid range", "bytes=0-499", fileSize, 0, 499, true},
		{"valid range middle", "bytes=200-799", fileSize, 200, 799, true},
		{"valid single byte", "bytes=0-0", fileSize, 0, 0, true},
		{"end exceeds file size", "bytes=0-1500", fileSize, 0, 999, true},
		{"end equals file size", "bytes=0-999", fileSize, 0, 999, true},
		{"start equals end", "bytes=500-500", fileSize, 500, 500, true},
		{"start beyond file size", "bytes=1000-1500", fileSize, 0, 0, false},
		{"end before start", "bytes=500-400", fileSize, 0, 0, false},
		{"negative start", "bytes=-100-500", fileSize, 0, 0, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, valid := handler._ParseRange(tc.rangeHeader, tc.fileSize)
			assert.Equal(t, tc.wantValid, valid, "valid mismatch")
			if valid {
				assert.Equal(t, tc.wantStart, start, "start mismatch")
				assert.Equal(t, tc.wantEnd, end, "end mismatch")
			}
		})
	}
}

// TestParseRange_FromPositionToEnd tests parsing of "bytes=start-" format
func TestParseRange_FromPositionToEnd(t *testing.T) {
	handler := NewStaticFilesHandlerTask("")
	fileSize := int64(1000)

	testCases := []struct {
		name        string
		rangeHeader string
		wantStart   int64
		wantEnd     int64
		wantValid   bool
	}{
		{"from beginning", "bytes=0-", 0, 999, true},
		{"from middle", "bytes=500-", 500, 999, true},
		{"from last byte", "bytes=999-", 999, 999, true},
		{"from beyond file", "bytes=1000-", 0, 0, false},
		{"negative start", "bytes=-100-", 0, 0, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, valid := handler._ParseRange(tc.rangeHeader, fileSize)
			assert.Equal(t, tc.wantValid, valid, "valid mismatch")
			if valid {
				assert.Equal(t, tc.wantStart, start, "start mismatch")
				assert.Equal(t, tc.wantEnd, end, "end mismatch")
			}
		})
	}
}

// TestParseRange_LastNBytes tests parsing of "bytes=-suffix" format
func TestParseRange_LastNBytes(t *testing.T) {
	handler := NewStaticFilesHandlerTask("")
	fileSize := int64(1000)

	testCases := []struct {
		name        string
		rangeHeader string
		wantStart   int64
		wantEnd     int64
		wantValid   bool
	}{
		{"last 500 bytes", "bytes=-500", 500, 999, true},
		{"last 1 byte", "bytes=-1", 999, 999, true},
		{"last bytes exceed file", "bytes=-2000", 0, 999, true},
		{"last bytes equal file", "bytes=-1000", 0, 999, true},
		{"zero suffix", "bytes=-0", 0, 0, false},
		{"negative suffix", "bytes=--100", 0, 0, false},
		{"empty suffix", "bytes=-", 0, 0, false},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, valid := handler._ParseRange(tc.rangeHeader, fileSize)
			assert.Equal(t, tc.wantValid, valid, "valid mismatch")
			if valid {
				assert.Equal(t, tc.wantStart, start, "start mismatch")
				assert.Equal(t, tc.wantEnd, end, "end mismatch")
			}
		})
	}
}

// TestParseRange_InvalidFormats tests invalid Range header formats
func TestParseRange_InvalidFormats(t *testing.T) {
	handler := NewStaticFilesHandlerTask("")
	fileSize := int64(1000)

	testCases := []struct {
		name        string
		rangeHeader string
	}{
		{"wrong unit", "characters=0-499"},
		{"no unit", "0-499"},
		{"multiple ranges", "bytes=0-100,200-300"},
		{"too many dashes", "bytes=0-100-200"},
		{"no dash", "bytes=0"},
		{"invalid characters", "bytes=abc-def"},
		{"empty header", ""},
		{"only equals", "bytes="},
		{"space in range", "bytes=0 - 100"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			_, _, valid := handler._ParseRange(tc.rangeHeader, fileSize)
			assert.False(t, valid, "should be invalid")
		})
	}
}

// TestParseRange_EdgeCases tests edge cases and boundary conditions
func TestParseRange_EdgeCases(t *testing.T) {
	handler := NewStaticFilesHandlerTask("")

	testCases := []struct {
		name        string
		rangeHeader string
		fileSize    int64
		wantStart   int64
		wantEnd     int64
		wantValid   bool
	}{
		{"empty file - valid range", "bytes=0-0", 1, 0, 0, true},
		{"empty file - invalid range", "bytes=0-1", 0, 0, 0, false},
		{"large file size", "bytes=0-999999", 1000000, 0, 999999, true},
		{"end exactly at boundary", "bytes=0-999", 1000, 0, 999, true},
		{"start at last byte", "bytes=999-999", 1000, 999, 999, true},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			start, end, valid := handler._ParseRange(tc.rangeHeader, tc.fileSize)
			assert.Equal(t, tc.wantValid, valid, "valid mismatch")
			if valid {
				assert.Equal(t, tc.wantStart, start, "start mismatch")
				assert.Equal(t, tc.wantEnd, end, "end mismatch")
			}
		})
	}
}

// TestParseRange_DataIntegrity verifies the integrity of parsed range values
func TestParseRange_DataIntegrity(t *testing.T) {
	handler := NewStaticFilesHandlerTask("")
	fileSize := int64(1000)

	// Test that start <= end always holds for valid ranges
	validRanges := []string{
		"bytes=0-999",
		"bytes=500-500",
		"bytes=100-200",
		"bytes=0-",
		"bytes=500-",
		"bytes=-100",
		"bytes=-1",
	}

	for _, rangeHeader := range validRanges {
		t.Run(rangeHeader, func(t *testing.T) {
			start, end, valid := handler._ParseRange(rangeHeader, fileSize)
			if valid {
				assert.True(t, start <= end, "start should be <= end")
				assert.True(t, start >= 0, "start should be >= 0")
				assert.True(t, end < fileSize, "end should be < fileSize")
				assert.True(t, end-start+1 > 0, "range length should be positive")
			}
		})
	}
}

// TestStaticFilesHandlerTask_RealFileRangeRequest_Integration performs ACTUAL integration testing
// This creates real files, loads them, and validates actual range processing
func TestStaticFilesHandlerTask_RealFileRangeRequest_Integration(t *testing.T) {
	// Create temporary directory and test file
	tmpDir := t.TempDir()
	testFileName := "integration_test_file.txt"
	testFilePath := tmpDir + "/" + testFileName

	// Create test content - 100 bytes of known data
	testContent := make([]byte, 100)
	for i := 0; i < 100; i++ {
		testContent[i] = byte('A' + (i % 26))
	}

	err := os.WriteFile(testFilePath, testContent, 0644)
	assert.NoError(t, err, "should create test file")

	// Verify file was created correctly
	fileInfo, err := os.Stat(testFilePath)
	assert.NoError(t, err, "should stat test file")
	assert.Equal(t, int64(100), fileInfo.Size(), "file size should be 100 bytes")

	t.Logf("✓ Created test file: %s (100 bytes)", testFilePath)

	// Create handler with no minify/cache for predictable testing
	handler := NewStaticFilesHandlerTask(tmpDir)
	handler.DoMinify = false
	handler.DoCache = false

	t.Run("Load file and verify content", func(t *testing.T) {
		// Test _Load method directly
		entity, err := handler._Load(testFilePath)

		assert.NoError(t, err, "should load file without error")
		assert.NotNil(t, entity, "entity should not be nil")
		assert.Equal(t, testContent, entity.data, "loaded data should match file content")
		assert.Equal(t, 100, len(entity.data), "loaded data should be 100 bytes")

		t.Logf("✓ File loaded successfully: %d bytes", len(entity.data))
	})

	t.Run("Parse and extract range: first 10 bytes", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=0-9", 100)
		assert.True(t, valid, "range should be valid")
		assert.Equal(t, int64(0), start, "start should be 0")
		assert.Equal(t, int64(9), end, "end should be 9")

		entity, err := handler._Load(testFilePath)
		assert.NoError(t, err, "should load file")
		rangeData := entity.data[start : end+1]
		assert.Equal(t, []byte("ABCDEFGHIJ"), rangeData, "range data should be ABCDEFGHIJ")

		t.Logf("✓ Range 0-9: extracted %d bytes: %s", len(rangeData), string(rangeData))
	})

	t.Run("Parse and extract range: middle 10 bytes", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=45-54", 100)
		assert.True(t, valid, "range should be valid")
		assert.Equal(t, int64(45), start, "start should be 45")
		assert.Equal(t, int64(54), end, "end should be 54")

		entity, err := handler._Load(testFilePath)
		assert.NoError(t, err, "should load file")
		rangeData := entity.data[start : end+1]
		assert.Equal(t, []byte("TUVWXYZABC"), rangeData, "range data should be TUVWXYZABC")

		t.Logf("✓ Range 45-54: extracted %d bytes: %s", len(rangeData), string(rangeData))
	})

	t.Run("Parse and extract range: from position to end", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=90-", 100)
		assert.True(t, valid, "range should be valid")
		assert.Equal(t, int64(90), start, "start should be 90")
		assert.Equal(t, int64(99), end, "end should be 99")

		entity, err := handler._Load(testFilePath)
		assert.NoError(t, err, "should load file")
		rangeData := entity.data[start : end+1]
		// bytes 90-99: M(90) N(91) O(92) P(93) Q(94) R(95) S(96) T(97) U(98) V(99)
		assert.Equal(t, []byte("MNOPQRSTUV"), rangeData, "range data should be MNOPQRSTUV")

		t.Logf("✓ Range 90-: extracted %d bytes: %s", len(rangeData), string(rangeData))
	})

	t.Run("Parse and extract range: last 5 bytes", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=-5", 100)
		assert.True(t, valid, "range should be valid")
		assert.Equal(t, int64(95), start, "start should be 95")
		assert.Equal(t, int64(99), end, "end should be 99")

		entity, err := handler._Load(testFilePath)
		assert.NoError(t, err, "should load file")
		rangeData := entity.data[start : end+1]
		// bytes 95-99: R(95) S(96) T(97) U(98) V(99)
		assert.Equal(t, []byte("RSTUV"), rangeData, "range data should be RSTUV")

		t.Logf("✓ Range -5: extracted %d bytes: %s", len(rangeData), string(rangeData))
	})

	t.Run("Parse and extract range: single byte", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=50-50", 100)
		assert.True(t, valid, "range should be valid")
		assert.Equal(t, int64(50), start, "start should be 50")
		assert.Equal(t, int64(50), end, "end should be 50")

		entity, err := handler._Load(testFilePath)
		assert.NoError(t, err, "should load file")
		rangeData := entity.data[start : end+1]
		assert.Equal(t, []byte("Y"), rangeData, "range data should be Y")

		t.Logf("✓ Range 50-50: extracted %d byte: %s", len(rangeData), string(rangeData))
	})

	t.Run("Parse and extract range: end exceeds file size", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=90-150", 100)
		assert.True(t, valid, "range should be valid (adjusted)")
		assert.Equal(t, int64(90), start, "start should be 90")
		assert.Equal(t, int64(99), end, "end should be adjusted to 99")

		entity, err := handler._Load(testFilePath)
		assert.NoError(t, err, "should load file")
		rangeData := entity.data[start : end+1]
		assert.Equal(t, 10, len(rangeData), "should extract 10 bytes")

		t.Logf("✓ Range 90-150 (adjusted to 90-99): extracted %d bytes", len(rangeData))
	})

	t.Run("Parse range: invalid - start beyond file", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=150-200", 100)
		assert.False(t, valid, "range should be invalid")

		t.Logf("✓ Invalid range 150-200: correctly rejected (start=%d, end=%d, valid=%v)", start, end, valid)
	})

	t.Run("Parse range: invalid - multiple ranges", func(t *testing.T) {
		start, end, valid := handler._ParseRange("bytes=0-10,20-30", 100)
		assert.False(t, valid, "range should be invalid")

		t.Logf("✓ Invalid multi-range: correctly rejected (start=%d, end=%d, valid=%v)", start, end, valid)
	})

	t.Run("Parse range: invalid - wrong unit", func(t *testing.T) {
		start, end, valid := handler._ParseRange("lines=0-10", 100)
		assert.False(t, valid, "range should be invalid")

		t.Logf("✓ Invalid unit 'lines': correctly rejected (start=%d, end=%d, valid=%v)", start, end, valid)
	})

	t.Run("Load non-existent file", func(t *testing.T) {
		entity, err := handler._Load(tmpDir + "/nonexistent.txt")
		assert.Nil(t, entity, "entity should be nil")
		assert.Nil(t, err, "error should be nil for non-existent file")

		t.Logf("✓ Non-existent file: correctly handled (entity=%v, err=%v)", entity, err)
	})
}

// TestStaticFilesHandlerTask_FullHTTPServerClient_Integration performs COMPLETE HTTP server/client testing
// This test actually starts an HTTP server, makes real HTTP requests, and validates full Range support
func TestStaticFilesHandlerTask_FullHTTPServerClient_Integration(t *testing.T) {
	// Create temporary directory and test files
	tmpDir := t.TempDir()

	// Create test file 1: 1000 bytes
	testFile1 := tmpDir + "/test1.txt"
	testContent1 := make([]byte, 1000)
	for i := 0; i < 1000; i++ {
		testContent1[i] = byte('A' + (i % 26))
	}
	err := os.WriteFile(testFile1, testContent1, 0644)
	require.NoError(t, err, "should create test file 1")

	// Create test file 2: 5000 bytes (larger file)
	testFile2 := tmpDir + "/test2.bin"
	testContent2 := make([]byte, 5000)
	for i := 0; i < 5000; i++ {
		testContent2[i] = byte(i % 256)
	}
	err = os.WriteFile(testFile2, testContent2, 0644)
	require.NoError(t, err, "should create test file 2")

	t.Logf("✓ Created test files: %s (1000 bytes), %s (5000 bytes)", testFile1, testFile2)

	// Find available port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err, "should get available port")
	port := listener.Addr().(*net.TCPAddr).Port
	listener.Close()

	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)
	t.Logf("✓ Using port: %d, base URL: %s", port, baseURL)

	// Create HTTP server with StaticFilesHandlerTask
	handler := NewStaticFilesHandlerTask(tmpDir)
	handler.DoMinify = false
	handler.DoCache = false

	// Create HTTP server
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Create wrapped Request
		req := &Request{
			request: r,
		}

		// Create wrapped Response
		resp := NewResponse(req)
		respWrapper := &testResponseWriter{w: w, resp: resp}

		// Call handler
		handler.Get(nil, req, resp, nil)

		// Simulate automatic Range handling (since we're bypassing DispatchHandler)
		if resp.body != nil && resp.body.ReadableBytes() > 0 {
			rangeHeader := r.Header.Get("Range")
			if rangeHeader != "" {
				content := resp.body.Bytes()
				contentSize := int64(len(content))
				if start, end, valid := ParseRange(rangeHeader, contentSize); valid {
					rangeData := content[start : end+1]
					resp.SetStatusCode(206)
					resp.SetHeader("Content-Range", fmt.Sprintf("bytes %d-%d/%d", start, end, contentSize))
					resp.SetHeader("Content-Length", fmt.Sprintf("%d", len(rangeData)))
					resp.SetBody(buf.NewByteBuf(rangeData))
				} else {
					resp.SetStatusCode(416)
					resp.SetHeader("Content-Range", fmt.Sprintf("bytes */%d", contentSize))
				}
			}
			resp.SetHeader("Accept-Ranges", "bytes")
		}

		// Write response
		respWrapper.WriteResponse()
	})

	server := &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", port),
		Handler: mux,
	}

	// Start server in background
	serverStarted := make(chan bool)
	serverError := make(chan error, 1)
	go func() {
		listener, err := net.Listen("tcp", server.Addr)
		if err != nil {
			serverError <- err
			return
		}
		serverStarted <- true
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			serverError <- err
		}
	}()

	// Wait for server to start or fail
	select {
	case <-serverStarted:
		t.Logf("✓ HTTP server started on %s", server.Addr)
	case err := <-serverError:
		t.Fatalf("Failed to start server: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("Server start timeout")
	}

	// Ensure server shutdown
	defer func() {
		if err := server.Close(); err != nil {
			t.Logf("Server close error: %v", err)
		}
		t.Logf("✓ HTTP server stopped")
	}()

	// Create HTTP client
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Wait a bit for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Test 1: Normal request without Range header
	t.Run("Full HTTP: Normal request", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/test1.txt")
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"), "should support ranges")
		assert.Equal(t, "1000", resp.Header.Get("Content-Length"), "content length should be 1000")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1, body, "body should match file content")

		t.Logf("✓ Normal request: status=%d, content-length=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Length"), len(body))
	})

	// Test 2: Range request - first 100 bytes
	t.Run("Full HTTP: Range 0-99", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=0-99")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 0-99/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "100", resp.Header.Get("Content-Length"), "content length should be 100")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[0:100], body, "body should match range")
		assert.Equal(t, 100, len(body), "should receive exactly 100 bytes")

		t.Logf("✓ Range 0-99: status=%d, content-range=%s, body-size=%d, data=%s...",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body), string(body[:10]))
	})

	// Test 3: Range request - middle bytes
	t.Run("Full HTTP: Range 500-599", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=500-599")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 500-599/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "100", resp.Header.Get("Content-Length"), "content length should be 100")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[500:600], body, "body should match range")

		t.Logf("✓ Range 500-599: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 4: Range request - from position to end
	t.Run("Full HTTP: Range 900-", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=900-")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 900-999/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "100", resp.Header.Get("Content-Length"), "content length should be 100")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[900:1000], body, "body should match range")
		assert.Equal(t, 100, len(body), "should receive exactly 100 bytes")

		t.Logf("✓ Range 900-: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 5: Range request - last N bytes
	t.Run("Full HTTP: Range -50", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=-50")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 950-999/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "50", resp.Header.Get("Content-Length"), "content length should be 50")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[950:1000], body, "body should match last 50 bytes")
		assert.Equal(t, 50, len(body), "should receive exactly 50 bytes")

		t.Logf("✓ Range -50: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 6: Range request - single byte
	t.Run("Full HTTP: Range single byte", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=100-100")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 100-100/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "1", resp.Header.Get("Content-Length"), "content length should be 1")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[100:101], body, "body should match single byte")
		assert.Equal(t, 1, len(body), "should receive exactly 1 byte")

		t.Logf("✓ Range 100-100: status=%d, content-range=%s, body-size=%d, byte=%c",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body), body[0])
	})

	// Test 7: Range request - larger file with multiple ranges
	t.Run("Full HTTP: Large file Range 4000-4999", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test2.bin", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=4000-4999")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 4000-4999/5000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "1000", resp.Header.Get("Content-Length"), "content length should be 1000")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent2[4000:5000], body, "body should match range")
		assert.Equal(t, 1000, len(body), "should receive exactly 1000 bytes")

		t.Logf("✓ Large file range 4000-4999: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 8: Invalid range request - beyond file size
	t.Run("Full HTTP: Invalid range beyond file", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=2000-3000")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 416, resp.StatusCode, "should return 416 Range Not Satisfiable")
		assert.Equal(t, "bytes */1000", resp.Header.Get("Content-Range"), "content range should show total size")

		t.Logf("✓ Invalid range: status=%d, content-range=%s",
			resp.StatusCode, resp.Header.Get("Content-Range"))
	})

	// Test 9: Invalid range request - multiple ranges
	t.Run("Full HTTP: Invalid multiple ranges", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=0-100,200-300")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 416, resp.StatusCode, "should return 416 Range Not Satisfiable")

		t.Logf("✓ Multiple ranges rejected: status=%d", resp.StatusCode)
	})

	// Test 10: File not found
	t.Run("Full HTTP: File not found", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/nonexistent.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=0-100")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 404, resp.StatusCode, "should return 404 Not Found")

		t.Logf("✓ File not found: status=%d", resp.StatusCode)
	})

	// Test 11: Verify actual data integrity with multiple requests
	t.Run("Full HTTP: Data integrity check", func(t *testing.T) {
		// Request multiple chunks and verify they match the original
		chunks := []struct {
			start int
			end   int
		}{
			{0, 99},
			{100, 199},
			{200, 299},
			{300, 399},
			{400, 499},
		}

		var reconstructed []byte

		for _, chunk := range chunks {
			req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
			require.NoError(t, err, "should create request")
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunk.start, chunk.end))

			resp, err := client.Do(req)
			require.NoError(t, err, "should make request")

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.NoError(t, err, "should read body")

			reconstructed = append(reconstructed, body...)
		}

		assert.Equal(t, testContent1[0:500], reconstructed, "reconstructed data should match original")
		assert.Equal(t, 500, len(reconstructed), "should have 500 bytes total")

		t.Logf("✓ Data integrity verified: reconstructed %d bytes from %d chunks",
			len(reconstructed), len(chunks))
	})
}

// testResponseWriter wraps http.ResponseWriter and Response for testing
type testResponseWriter struct {
	w    http.ResponseWriter
	resp *Response
}

func (trw *testResponseWriter) WriteResponse() {
	// Write headers first (must be before WriteHeader)
	for key, values := range trw.resp.header {
		for _, value := range values {
			trw.w.Header().Set(key, value)
		}
	}

	// Write status code
	statusCode := trw.resp.statusCode
	if statusCode == 0 {
		statusCode = 200
	}
	trw.w.WriteHeader(statusCode)

	// Write body
	if trw.resp.body != nil {
		trw.w.Write(trw.resp.body.Bytes())
	}
}

// TestStaticFilesHandlerTask_GoneHTTPServerClient_Integration performs REAL Gone HTTP server/client testing
// This test follows the pattern from example/ghttp/server_test.go to properly start a Gone HTTP server
func TestStaticFilesHandlerTask_GoneHTTPServerClient_Integration(t *testing.T) {
	kklogger.SetLogLevel("DEBUG")

	// Create temporary directory and test files
	tmpDir := t.TempDir()

	// Create test file 1: 1000 bytes
	testFile1 := tmpDir + "/test1.txt"
	testContent1 := make([]byte, 1000)
	for i := 0; i < 1000; i++ {
		testContent1[i] = byte('A' + (i % 26))
	}
	err := os.WriteFile(testFile1, testContent1, 0644)
	require.NoError(t, err, "should create test file 1")

	// Create test file 2: 5000 bytes
	testFile2 := tmpDir + "/test2.bin"
	testContent2 := make([]byte, 5000)
	for i := 0; i < 5000; i++ {
		testContent2[i] = byte(i % 256)
	}
	err = os.WriteFile(testFile2, testContent2, 0644)
	require.NoError(t, err, "should create test file 2")

	t.Logf("✓ Created test files: %s (1000 bytes), %s (5000 bytes)", testFile1, testFile2)

	// Create static files handler
	staticHandler := NewStaticFilesHandlerTask(tmpDir)
	staticHandler.DoMinify = false
	staticHandler.DoCache = false

	// Create route with specific file endpoints (concrete testing approach)
	route := NewSimpleRoute()
	route.SetEndpoint("/test1.txt", staticHandler)
	route.SetEndpoint("/test2.bin", staticHandler)
	route.SetEndpoint("/nonexistent.txt", staticHandler)

	// Setup Gone HTTP server using channel.ServerBootstrap
	bootstrap := channel.NewServerBootstrap()
	bootstrap.ChannelType(&ServerChannel{})
	bootstrap.Handler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	bootstrap.ChildHandler(channel.NewInitializer(func(ch channel.Channel) {
		ch.Pipeline().AddLast("INDICATE_HANDLER_INBOUND", &channel.IndicateHandlerInbound{})
		ch.Pipeline().AddLast("NET_STATUS_INBOUND", &channel.NetStatusInbound{})
		ch.Pipeline().AddLast("LOG_HANDLER", NewLogHandler(false))
		ch.Pipeline().AddLast("DISPATCHER", NewDispatchHandler(route))
		ch.Pipeline().AddLast("NET_STATUS_OUTBOUND", &channel.NetStatusOutbound{})
		ch.Pipeline().AddLast("INDICATE_HANDLER_OUTBOUND", &channel.IndicateHandlerOutbound{})
	}))

	// Bind to port 18085 (following example/ghttp pattern)
	testPort := 18085
	serverCh := bootstrap.Bind(&net.TCPAddr{IP: nil, Port: testPort}).Sync().Channel()
	require.NotNil(t, serverCh, "server channel should not be nil")

	// Use fixed port address
	baseURL := fmt.Sprintf("http://localhost:%d", testPort)
	t.Logf("✓ Gone HTTP server started on port %d", testPort)

	// Ensure server shutdown
	defer func() {
		serverCh.Close()
		serverCh.CloseFuture().Sync()
		t.Logf("✓ Gone HTTP server stopped")
	}()

	// Create HTTP client
	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	// Wait for server to be ready
	time.Sleep(100 * time.Millisecond)

	// Test 1: Normal request without Range header
	t.Run("Gone Server: Normal request", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/test1.txt")
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode, "should return 200 OK")
		assert.Equal(t, "bytes", resp.Header.Get("Accept-Ranges"), "should support ranges")
		assert.Equal(t, "1000", resp.Header.Get("Content-Length"), "content length should be 1000")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1, body, "body should match file content")

		t.Logf("✓ Normal request: status=%d, content-length=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Length"), len(body))
	})

	// Test 2: Range request - first 100 bytes
	t.Run("Gone Server: Range 0-99", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=0-99")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 0-99/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "100", resp.Header.Get("Content-Length"), "content length should be 100")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[0:100], body, "body should match range")
		assert.Equal(t, 100, len(body), "should receive exactly 100 bytes")

		t.Logf("✓ Range 0-99: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 3: Range request - middle bytes
	t.Run("Gone Server: Range 500-599", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=500-599")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 500-599/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "100", resp.Header.Get("Content-Length"), "content length should be 100")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[500:600], body, "body should match range")

		t.Logf("✓ Range 500-599: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 4: Range request - from position to end
	t.Run("Gone Server: Range 900-", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=900-")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 900-999/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "100", resp.Header.Get("Content-Length"), "content length should be 100")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[900:1000], body, "body should match range")
		assert.Equal(t, 100, len(body), "should receive exactly 100 bytes")

		t.Logf("✓ Range 900-: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 5: Range request - last N bytes
	t.Run("Gone Server: Range -50", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=-50")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 950-999/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "50", resp.Header.Get("Content-Length"), "content length should be 50")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[950:1000], body, "body should match last 50 bytes")
		assert.Equal(t, 50, len(body), "should receive exactly 50 bytes")

		t.Logf("✓ Range -50: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 6: Range request - single byte
	t.Run("Gone Server: Range single byte", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=100-100")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 100-100/1000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "1", resp.Header.Get("Content-Length"), "content length should be 1")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent1[100:101], body, "body should match single byte")
		assert.Equal(t, 1, len(body), "should receive exactly 1 byte")

		t.Logf("✓ Range 100-100: status=%d, content-range=%s, body-size=%d, byte=%c",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body), body[0])
	})

	// Test 7: Range request - larger file
	t.Run("Gone Server: Large file Range 4000-4999", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test2.bin", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=4000-4999")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 206, resp.StatusCode, "should return 206 Partial Content")
		assert.Equal(t, "bytes 4000-4999/5000", resp.Header.Get("Content-Range"), "content range should be correct")
		assert.Equal(t, "1000", resp.Header.Get("Content-Length"), "content length should be 1000")

		body, err := io.ReadAll(resp.Body)
		require.NoError(t, err, "should read body")
		assert.Equal(t, testContent2[4000:5000], body, "body should match range")
		assert.Equal(t, 1000, len(body), "should receive exactly 1000 bytes")

		t.Logf("✓ Large file range 4000-4999: status=%d, content-range=%s, body-size=%d",
			resp.StatusCode, resp.Header.Get("Content-Range"), len(body))
	})

	// Test 8: Invalid range - beyond file size
	t.Run("Gone Server: Invalid range beyond file", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=2000-3000")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 416, resp.StatusCode, "should return 416 Range Not Satisfiable")
		assert.Equal(t, "bytes */1000", resp.Header.Get("Content-Range"), "content range should show total size")

		t.Logf("✓ Invalid range: status=%d, content-range=%s",
			resp.StatusCode, resp.Header.Get("Content-Range"))
	})

	// Test 9: Invalid range - multiple ranges
	t.Run("Gone Server: Invalid multiple ranges", func(t *testing.T) {
		req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
		require.NoError(t, err, "should create request")
		req.Header.Set("Range", "bytes=0-100,200-300")

		resp, err := client.Do(req)
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 416, resp.StatusCode, "should return 416 Range Not Satisfiable")

		t.Logf("✓ Multiple ranges rejected: status=%d", resp.StatusCode)
	})

	// Test 10: File not found
	t.Run("Gone Server: File not found", func(t *testing.T) {
		resp, err := client.Get(baseURL + "/nonexistent.txt")
		require.NoError(t, err, "should make request")
		defer resp.Body.Close()

		assert.Equal(t, 404, resp.StatusCode, "should return 404 Not Found")

		t.Logf("✓ File not found: status=%d", resp.StatusCode)
	})

	// Test 11: Data integrity check with multiple chunks
	t.Run("Gone Server: Data integrity check", func(t *testing.T) {
		chunks := []struct {
			start int
			end   int
		}{
			{0, 99},
			{100, 199},
			{200, 299},
			{300, 399},
			{400, 499},
		}

		var reconstructed []byte

		for _, chunk := range chunks {
			req, err := http.NewRequest("GET", baseURL+"/test1.txt", nil)
			require.NoError(t, err, "should create request")
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-%d", chunk.start, chunk.end))

			resp, err := client.Do(req)
			require.NoError(t, err, "should make request")

			body, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			require.NoError(t, err, "should read body")

			reconstructed = append(reconstructed, body...)
		}

		assert.Equal(t, testContent1[0:500], reconstructed, "reconstructed data should match original")
		assert.Equal(t, 500, len(reconstructed), "should have 500 bytes total")

		t.Logf("✓ Data integrity verified: reconstructed %d bytes from %d chunks",
			len(reconstructed), len(chunks))
	})
}
