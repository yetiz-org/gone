package ghttp

import "testing"

func TestHTTPMethodConstants(t *testing.T) {
	tests := map[string]string{
		MethodGet:     "GET",
		MethodHead:    "HEAD",
		MethodPost:    "POST",
		MethodPut:     "PUT",
		MethodPatch:   "PATCH",
		MethodDelete:  "DELETE",
		MethodConnect: "CONNECT",
		MethodOptions: "OPTIONS",
		MethodTrace:   "TRACE",
	}

	for got, want := range tests {
		if got != want {
			t.Fatalf("method constant = %q, want %q", got, want)
		}
	}
}
