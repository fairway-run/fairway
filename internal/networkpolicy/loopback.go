package networkpolicy

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"time"
)

type LookupIPAddr func(context.Context, string) ([]net.IPAddr, error)

// NewLoopbackHTTPClient returns an HTTP client that ignores proxy environment
// variables, rejects redirects, and refuses any connection whose complete DNS
// result is not loopback. When allowDNS is false, hostnames are rejected before
// resolver access and only numeric loopback addresses are accepted.
func NewLoopbackHTTPClient(timeout time.Duration, allowDNS bool) *http.Client {
	return newLoopbackHTTPClient(timeout, allowDNS, net.DefaultResolver.LookupIPAddr)
}

func newLoopbackHTTPClient(timeout time.Duration, allowDNS bool, lookup LookupIPAddr) *http.Client {
	return &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			Proxy:       nil,
			DialContext: loopbackDialContext(allowDNS, lookup),
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("loopback HTTP redirects are disabled")
		},
	}
}

func loopbackDialContext(allowDNS bool, lookup LookupIPAddr) func(context.Context, string, string) (net.Conn, error) {
	return func(ctx context.Context, network, address string) (net.Conn, error) {
		host, port, err := net.SplitHostPort(address)
		if err != nil {
			return nil, fmt.Errorf("loopback endpoint address is invalid")
		}
		ip := net.ParseIP(host)
		if ip != nil {
			if !ip.IsLoopback() {
				return nil, fmt.Errorf("loopback endpoint resolved outside loopback")
			}
			return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		}
		if !allowDNS {
			return nil, fmt.Errorf("loopback endpoint DNS is disabled")
		}
		addresses, err := lookup(ctx, host)
		if err != nil || len(addresses) == 0 {
			return nil, fmt.Errorf("loopback endpoint resolution failed")
		}
		for _, resolved := range addresses {
			if !resolved.IP.IsLoopback() {
				return nil, fmt.Errorf("loopback endpoint resolved outside loopback")
			}
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(ctx, network, net.JoinHostPort(addresses[0].IP.String(), port))
	}
}
