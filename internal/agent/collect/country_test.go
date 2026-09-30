package collect

import "testing"

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
