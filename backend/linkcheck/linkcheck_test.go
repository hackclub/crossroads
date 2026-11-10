package linkcheck

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestPublicIP(t *testing.T) {
	private := []string{"127.0.0.1", "127.1.2.3", "10.0.0.5", "172.16.0.1", "172.31.255.255", "192.168.1.1",
		"169.254.169.254", "0.0.0.0", "100.64.0.1", "::1", "fe80::1", "fd00::1", "::ffff:10.0.0.1", "224.0.0.1"}
	for _, s := range private {
		if publicIP(net.ParseIP(s)) {
			t.Errorf("%s counted as public", s)
		}
	}
	for _, s := range []string{"8.8.8.8", "140.82.112.3", "2606:4700::1111", "172.32.0.1", "100.128.0.1"} {
		if !publicIP(net.ParseIP(s)) {
			t.Errorf("%s counted as private", s)
		}
	}
}

func TestInvalidRefusesInternalAddresses(t *testing.T) {
	for _, link := range []string{"http://localhost:3000/", "http://127.0.0.1/", "http://10.0.0.5/admin", "http://[::1]/",
		"http://169.254.169.254/latest/meta-data/", "http://0.0.0.0/", "http://192.168.0.1/"} {
		if Invalid(link) == "" {
			t.Errorf("%s passed", link)
		}
	}
	for _, link := range []string{"https://example.com/", "https://my-app.fly.dev/play", "http://8.8.8.8/"} {
		if p := Invalid(link); p != "" {
			t.Errorf("%s refused: %s", link, p)
		}
	}
}

// A server on this machine stands in for anything inside the network: the
// checker must refuse to fetch it. Literal addresses and "localhost" are
// refused up front; a name is refused after DNS, at dial time, which is the
// check a public name that resolves inside the network, or a redirect, hits.
func TestCheckNeverReachesInternalAddresses(t *testing.T) {
	reached := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { reached = true }))
	defer srv.Close()
	c := New()
	if r := c.Check(context.Background(), srv.URL); r.State != Broken {
		t.Errorf("direct: %+v", r)
	}
	port := srv.URL[strings.LastIndex(srv.URL, ":")+1:]
	conn, err := dialPublic(context.Background(), "tcp", "localhost:"+port)
	if conn != nil {
		conn.Close()
	}
	if !errors.Is(err, ErrNotPublic) {
		t.Errorf("dialing a name that resolves to loopback: got %v, want ErrNotPublic", err)
	}
	if reached {
		t.Fatal("the checker connected to an internal address")
	}
}

func TestDevWithoutARealChecker(t *testing.T) {
	d := Dev{}
	if r := d.Check(context.Background(), "https://dev-ana.example.com/x"); r.State != OK {
		t.Errorf("made-up link: %+v", r)
	}
	if r := d.Check(context.Background(), "https://real.example.com/x"); r.State != Unknown {
		t.Errorf("real link without a checker: %+v", r)
	}
}
