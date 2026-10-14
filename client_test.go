package zeroclaw

import (
	"errors"
	"fmt"
	"net/url"
	"strings"
	"testing"
)

func TestURLVectors(t *testing.T) {
	vectors := []struct {
		base     string
		segments []string
		query    url.Values
		want     string
	}{
		{"http://gw:42617", []string{"health"}, nil, "http://gw:42617/health"},
		{"http://gw:42617/", []string{"api", "status"}, nil, "http://gw:42617/api/status"},
		{"https://host/zc/", []string{"api", "cron", "j 1"}, nil, "https://host/zc/api/cron/j%201"},
		{"http://gw", []string{"api", "memory", "a/b"}, nil, "http://gw/api/memory/a%2Fb"},
		{"http://gw", []string{"api", "memory", "k?x#y"}, nil, "http://gw/api/memory/k%3Fx%23y"},
		{"http://gw", []string{"api", "channels", "whatsapp.main", "rooms", "1203@g.us", "members", "+34600"}, nil,
			"http://gw/api/channels/whatsapp.main/rooms/1203@g.us/members/+34600"},
		{"http://gw", []string{"api", "config", "prop"}, params("path", "a.b c"), "http://gw/api/config/prop?path=a.b+c"},
		{"http://gw", []string{"api", "logs"}, params("q", "a&b=c", "limit", "5"), "http://gw/api/logs?limit=5&q=a%26b%3Dc"},
	}
	for _, v := range vectors {
		client, err := New(v.base)
		if err != nil {
			t.Fatal(err)
		}
		if got := client.url(v.segments, v.query).String(); got != v.want {
			t.Errorf("%s %v: %s, want %s", v.base, v.segments, got, v.want)
		}
	}
}

func TestBaseURLVectors(t *testing.T) {
	vectors := map[string]bool{
		"http://127.0.0.1:42617":            true,
		"https://gateway.example/zeroclaw/": true,
		"ws://gateway":                      false,
		"ftp://gateway":                     false,
		"gateway:42617":                     false,
		"":                                  false,
		"mailto:x@example.com":              false,
		"not a url":                         false,
	}
	for base, valid := range vectors {
		_, err := New(base)
		if (err == nil) != valid {
			t.Errorf("%q: %v", base, err)
		}
		if err != nil && !errors.Is(err, ErrInvalidURL) {
			t.Errorf("%q: %v is not ErrInvalidURL", base, err)
		}
	}
}

func TestSegmentsAreEncodedUnderTheBasePath(t *testing.T) {
	client, _ := New("http://gw.local:42617/zeroclaw/")
	got := client.url([]string{"api", "memory", "a/b?c"}, params("agent", "x y")).String()
	if want := "http://gw.local:42617/zeroclaw/api/memory/a%2Fb%3Fc?agent=x+y"; got != want {
		t.Errorf("%s, want %s", got, want)
	}
}

func TestFormattingHidesTheToken(t *testing.T) {
	client, _ := New("http://x", WithToken("secret-token"))
	for _, shown := range []string{fmt.Sprint(client), fmt.Sprintf("%v %+v %#v %s", client, client, client, client)} {
		if strings.Contains(shown, "secret-token") || !strings.Contains(shown, "authenticated: true") {
			t.Errorf("%s", shown)
		}
	}
}

func TestUnsetParametersAreLeftOut(t *testing.T) {
	if got := params("a", "1", "b", "").Encode(); got != "a=1" {
		t.Errorf("%s", got)
	}
}

func TestWithTokenKeepsTheGateway(t *testing.T) {
	client, _ := New("http://gw/zc", WithToken("a"))
	other := client.WithToken("b")
	if other.token != "b" || client.token != "a" || other.BaseURL().String() != "http://gw/zc" || other.http != client.http {
		t.Errorf("%v %v", client, other)
	}
}
