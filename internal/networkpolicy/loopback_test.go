package networkpolicy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestLoopbackClientRejectsDirectRemoteAndDNS(t *testing.T) {
	var lookups atomic.Int64
	lookup := func(context.Context, string) ([]net.IPAddr, error) {
		lookups.Add(1)
		return []net.IPAddr{{IP: net.ParseIP("198.51.100.7")}}, nil
	}
	client := newLoopbackHTTPClient(time.Second, false, lookup)
	for _, endpoint := range []string{"http://198.51.100.7:8080", "http://remote.example:8080"} {
		_, err := client.Get(endpoint)
		if err == nil || !strings.Contains(err.Error(), "loopback") {
			t.Fatalf("GET %s error = %v, want loopback rejection", endpoint, err)
		}
	}
	if lookups.Load() != 0 {
		t.Fatalf("DNS lookup count = %d, want 0", lookups.Load())
	}

	client = newLoopbackHTTPClient(time.Second, true, lookup)
	if _, err := client.Get("http://remote.example:8080"); err == nil || !strings.Contains(err.Error(), "outside loopback") {
		t.Fatalf("DNS remote error = %v, want outside-loopback rejection", err)
	}
	if lookups.Load() != 1 {
		t.Fatalf("DNS lookup count = %d, want 1", lookups.Load())
	}
}

func TestLoopbackClientRejectsRedirectAndIgnoresProxyEnvironment(t *testing.T) {
	var proxyHits atomic.Int64
	proxy := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		proxyHits.Add(1)
	}))
	defer proxy.Close()
	t.Setenv("HTTP_PROXY", proxy.URL)
	t.Setenv("HTTPS_PROXY", proxy.URL)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirect" {
			http.Redirect(w, r, "http://198.51.100.9/escape", http.StatusFound)
			return
		}
		fmt.Fprint(w, "local")
	}))
	defer server.Close()

	client := NewLoopbackHTTPClient(time.Second, false)
	resp, err := client.Get(server.URL)
	if err != nil {
		t.Fatalf("loopback GET: %v", err)
	}
	_ = resp.Body.Close()
	if proxyHits.Load() != 0 {
		t.Fatalf("proxy hits = %d, want 0", proxyHits.Load())
	}
	if _, err := client.Get(server.URL + "/redirect"); err == nil || !strings.Contains(err.Error(), "redirects are disabled") {
		t.Fatalf("redirect error = %v, want disabled", err)
	}
}
