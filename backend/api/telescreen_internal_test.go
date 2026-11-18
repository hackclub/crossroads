package api

import "testing"

func TestTelescreenHackatimeURL(t *testing.T) {
	for _, c := range []struct {
		user     string
		projects []string
		want     string
	}{
		{"68321", []string{"Texel"}, "https://telescreen.hackclub.com/workbench/hackatime/overview?u=68321&p=Texel"},
		{"68321", []string{"Texel", "ViniClock", "Vinifolio"}, "https://telescreen.hackclub.com/workbench/hackatime/overview?u=68321&p=Texel,ViniClock,Vinifolio"},
		{"68321", []string{"My Game", "a&b"}, "https://telescreen.hackclub.com/workbench/hackatime/overview?u=68321&p=My%20Game,a%26b"},
		{"", []string{"Texel"}, ""},
		{"68321", nil, ""},
	} {
		if got := telescreenHackatimeURL(c.user, c.projects); got != c.want {
			t.Errorf("telescreenHackatimeURL(%q, %q) = %q, want %q", c.user, c.projects, got, c.want)
		}
	}
}
