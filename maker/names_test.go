package maker

import "testing"

func TestSafeName(t *testing.T) {
	cases := map[string]string{
		"anna":         "anna",
		`a\b/c:d*e?f`:  "a_b_c_d_e_f",
		`"<>|`:         "____",
		"tab\there":    "tab_here",
		"trailing. . ": "trailing",
		"...":          "launcher",
		"":             "launcher",
		"CON":          "_CON",
		"nul.txt":      "_nul.txt",
		"com1":         "_com1",
		"console":      "console",
		"::1":          "__1",
	}
	for in, want := range cases {
		if got := SafeName(in); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestFileBase(t *testing.T) {
	if got := FileBase(`CORP\anna`, "Krasna"); got != "anna_Krasna" {
		t.Fatalf("got %q", got)
	}
	if got := FileBase("oleh", "168.119.80.235"); got != "oleh_168.119.80.235" {
		t.Fatalf("got %q", got)
	}
}

func TestSameLogin(t *testing.T) {
	if !SameLogin(`CORP\Anna`, "anna") || SameLogin("anna", "oleh") {
		t.Fatal("SameLogin compares the bare login, case-insensitively")
	}
}
