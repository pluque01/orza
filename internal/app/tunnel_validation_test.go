package app

import (
	"errors"
	"testing"
)

func TestParseTunnelEndpoint(t *testing.T) {
	for _, tt := range []struct{ input, want string }{
		{"localhost:80", "localhost:80"}, {":65535", ":65535"}, {"[::1]:443", "[::1]:443"},
		{"service.example:123", "service.example:123"},
	} {
		e, err := ParseTunnelEndpoint(tt.input)
		if err != nil || e.String() != tt.want {
			t.Errorf("parse %q = %v, %v", tt.input, e, err)
		}
	}
	for _, input := range []string{"host:0", "host:65536", "host:-1", "host:+1", "host:http", "host: 80", "host:1\n", "::1:80", "[host]:80", "bad host:80", "bad_name:80", "-bad:80", "a..b:80", "host\x00:80", "host:1:2"} {
		if _, err := ParseTunnelEndpoint(input); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("accepted %q: %v", input, err)
		}
	}
}

func TestValidateTunnelConfig(t *testing.T) {
	base := TunnelConfig{Mode: TunnelLocal, Listen: TunnelEndpoint{Port: 8000}, Destination: TunnelEndpoint{Host: "service.example", Port: 80}}
	for _, host := range []string{"", "localhost", "127.0.0.1", "::1", "::ffff:127.0.0.1"} {
		c := base
		c.Listen.Host = host
		got, err := ValidateTunnelConfig(c)
		if err != nil {
			t.Errorf("loopback %q: %v", host, err)
		}
		if host == "" || host == "localhost" {
			if got.Listen.Host != "127.0.0.1" {
				t.Errorf("default = %q", got.Listen.Host)
			}
		}
	}
	for _, mode := range []TunnelMode{TunnelLocal, TunnelRemote, TunnelDynamic} {
		c := base
		c.Mode = mode
		if mode == TunnelDynamic {
			c.Destination = TunnelEndpoint{}
		}
		if _, err := ValidateTunnelConfig(c); err != nil {
			t.Errorf("mode %s: %v", mode, err)
		}
	}
	for _, change := range []func(*TunnelConfig){
		func(c *TunnelConfig) { c.Mode = "other" }, func(c *TunnelConfig) { c.Listen.Port = 0 },
		func(c *TunnelConfig) { c.Listen.Host = "example.com" }, func(c *TunnelConfig) { c.Listen.Host = "0.0.0.0" },
		func(c *TunnelConfig) { c.Destination = TunnelEndpoint{} }, func(c *TunnelConfig) { c.Destination.Host = "host:80" },
		func(c *TunnelConfig) { c.Destination.Host = "a\u202eb" }, func(c *TunnelConfig) { c.Mode = TunnelDynamic },
	} {
		c := base
		change(&c)
		if _, err := ValidateTunnelConfig(c); !errors.Is(err, ErrInvalidRequest) {
			t.Errorf("accepted %#v: %v", c, err)
		}
	}
	c := base
	c.Listen.Host = "::"
	c.ExposureAcknowledged = true
	if _, err := ValidateTunnelConfig(c); err != nil {
		t.Fatal(err)
	}
}
