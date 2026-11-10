package unified

import (
	"encoding/json"
	"slices"
	"testing"
	"time"

	"github.com/scherepi/crossroads-backend/db"
)

func TestNameProblem(t *testing.T) {
	ok := [][2]string{
		{"Ada", "Lovelace"}, {"José", "Núñez-García"}, {"Mary Jane", "O'Brien"}, {"Lê", "Văn Thành"},
		{"Zoë", "St. John"}, {"李", "小龙"}, {"J.", "Smith"},
	}
	for _, n := range ok {
		if p := NameProblem(n[0], n[1]); p != "" {
			t.Errorf("%q %q: %s", n[0], n[1], p)
		}
	}
	bad := [][2]string{
		{"", ""}, {"Ada", ""}, {"", "Lovelace"}, {"  ", "Lovelace"}, {"asdf", "Lovelace"}, {"Ada", "test"},
		{"Ada2", "Lovelace"}, {"ada@example.com", "x"}, {"Ada", "---"},
	}
	for _, n := range bad {
		if NameProblem(n[0], n[1]) == "" {
			t.Errorf("%q %q passed", n[0], n[1])
		}
	}
}

func TestMissing(t *testing.T) {
	birthday := time.Date(2009, 4, 1, 0, 0, 0, 0, time.UTC)
	address, _ := json.Marshal(db.Address{Line1: "1 Main St", City: "Shelburne", PostalCode: "05482", Country: "US"})
	user := db.User{FirstName: "Ada", LastName: "Lovelace", Email: "ada@example.com", Birthday: &birthday, Address: address}
	link := func(s string) *string { return &s }
	project := db.Project{Description: "A racing game", ScreenshotUrl: link("https://x.example/a.png"),
		RepoUrl: link("https://github.com/ada/racer"), DemoUrl: link("https://ada.example.com")}
	if m := Missing(user, project); len(m) != 0 {
		t.Fatalf("complete, but missing %v", m)
	}

	user.Birthday = nil
	user.Address, _ = json.Marshal(db.Address{Line1: "1 Main St", Country: "US"})
	project.RepoUrl = link("https://gitlab.com/ada/racer")
	project.ScreenshotUrl = nil
	got := Missing(user, project)
	for _, want := range []string{"a birthday", "a full address (no city, postal code)", "a screenshot",
		"a repo on GitHub (the Unified YSWS DB asks for a GitHub username)"} {
		if !slices.Contains(got, want) {
			t.Errorf("missing %v, doesn't include %q", got, want)
		}
	}
}

func TestNormalize(t *testing.T) {
	same := []string{
		"https://github.com/Ada/Racer", "http://www.github.com/ada/racer/", "https://github.com/ada/racer.git",
		"https://github.com/ada/racer/tree/main/src", "https://github.com/ada/racer?tab=readme#top",
	}
	for _, s := range same {
		if got := Normalize(s); got != "github.com/ada/racer" {
			t.Errorf("Normalize(%q) = %q", s, got)
		}
	}
	if !sameProject("ada.example.com/racer", "https://ada.example.com/racer/play") {
		t.Error("a page inside the demo isn't the same project")
	}
	if sameProject("github.com/ada/racer", "https://github.com/ada/racer-2") {
		t.Error("racer-2 is the same project as racer")
	}
}

func TestFormulaString(t *testing.T) {
	cases := map[string]string{
		`github.com/ada/racer`:         `github.com/ada/racer`,
		`example.com/app\`:             `example.com/app\\`,
		`example.com/a"b`:              `example.com/a\"b`,
		"example.com/a\x00b\nc":        `example.com/abc`,
		`example.com/\"), 1=1, FIND("`: `example.com/\\\"), 1=1, FIND(\"`,
	}
	for in, want := range cases {
		if got := formulaString(in); got != want {
			t.Errorf("formulaString(%q) = %q, want %q", in, got, want)
		}
	}
}
