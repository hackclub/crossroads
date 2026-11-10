package main

import "testing"

func TestRunsLocally(t *testing.T) {
	t.Setenv("KUBERNETES_SERVICE_HOST", "")
	for url, want := range map[string]bool{
		"http://localhost:8080/auth/hcacallback":           true,
		"http://LOCALHOST:5173/auth/hcacallback":           true,
		"http://127.0.0.1:8080/auth/hcacallback":           true,
		"http://[::1]:8080/auth/hcacallback":               true,
		"https://localhost:8080/auth/hcacallback":          false,
		"HTTPS://crossroads.hackclub.com/auth/hcacallback": false,
		"http://crossroads.hackclub.com/auth/hcacallback":  false,
		"http://192.168.1.20:8080/auth/hcacallback":        false,
		"http://localhost.evil.com/auth/hcacallback":       false,
		"": false,
	} {
		if got := runsLocally(url); got != want {
			t.Errorf("runsLocally(%q) = %v, want %v", url, got, want)
		}
	}

	t.Setenv("KUBERNETES_SERVICE_HOST", "10.96.0.1")
	if runsLocally("http://localhost:8080/auth/hcacallback") {
		t.Error("runsLocally is true inside Kubernetes")
	}
}
