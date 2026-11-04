package config

import "testing"

func TestHTTPAddressPrefersExplicitAddress(t *testing.T) {
	t.Setenv("HTTP_ADDR", "127.0.0.1:9000")
	t.Setenv("PORT", "7000")
	if got := httpAddress(); got != "127.0.0.1:9000" {
		t.Fatalf("got %q, want explicit HTTP_ADDR", got)
	}
}

func TestHTTPAddressUsesRailwayPort(t *testing.T) {
	t.Setenv("HTTP_ADDR", "")
	t.Setenv("PORT", "7000")
	if got := httpAddress(); got != ":7000" {
		t.Fatalf("got %q, want :7000", got)
	}
}
