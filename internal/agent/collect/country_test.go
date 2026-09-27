package collect

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseTraceLoc(t *testing.T) {
	cases := map[string]string{
		"fl=1\nloc=jp\nip=1.1.1.1": "JP",
		"loc=XX":                   "",
		"":                         "",
		"ip=1\r\nloc=US\r\n":       "US",
	}
	for in, want := range cases {
		if got := ParseTraceLoc(in); got != want {
			t.Errorf("ParseTraceLoc(%q) = %q，期望 %q", in, got, want)
		}
	}
}

func TestDetectCountry(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, "ip=1.1.1.1\nloc=US\n")
	}))
	defer srv.Close()
	cc, err := DetectCountry(context.Background(), srv.Client(), srv.URL)
	if err != nil || cc != "US" {
		t.Fatalf("DetectCountry = %q, %v", cc, err)
	}
}
