package semver

import "testing"

func TestParse(t *testing.T) {
	ok := map[string]Version{
		"1.2.3":              {1, 2, 3, ""},
		"v1.2.3":             {1, 2, 3, ""},
		" 1.2.3 ":            {1, 2, 3, ""},
		"0.1.0":              {0, 1, 0, ""},
		"1.0.0-rc.1":         {1, 0, 0, "rc.1"},
		"1.0.0-rc.1+build.5": {1, 0, 0, "rc.1"},
		"10.20.30":           {10, 20, 30, ""},
	}
	for in, want := range ok {
		got, err := Parse(in)
		if err != nil {
			t.Errorf("Parse(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("Parse(%q) = %+v, want %+v", in, got, want)
		}
	}

	bad := []string{
		"", "1", "1.2", "1.2.3.4", "one.2.3", "1.2.x", "-1.2.3", "1.-2.3",
		// Leading zeros are not semver, and accepting them makes two spellings
		// of one version compare equal here and differ as strings elsewhere.
		"01.2.3", "1.02.3", "1.2.03",
		// Atoi accepts a sign; a version component does not.
		"+1.2.3", "1.+2.3",
	}
	for _, in := range bad {
		if got, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) = %+v, want an error", in, got)
		}
	}
}

func TestCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "1.0.1", -1},
		{"1.0.0", "1.1.0", -1},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.9.9", 1},
		// A pre-release sorts before its release.
		{"1.0.0-rc.1", "1.0.0", -1},
		{"1.0.0", "1.0.0-rc.1", 1},
		// The case a lexical comparison gets wrong, and the reason this
		// function exists: rc.9 must come before rc.10.
		{"1.0.0-rc.9", "1.0.0-rc.10", -1},
		{"1.0.0-rc.10", "1.0.0-rc.9", 1},
		{"1.0.0-rc.2", "1.0.0-rc.11", -1},
		// A numeric identifier sorts below an alphanumeric one.
		{"1.0.0-1", "1.0.0-alpha", -1},
		{"1.0.0-alpha", "1.0.0-1", 1},
		// A shorter set of identifiers sorts below a longer one with the same
		// prefix. This is the semver 2.0 example, kept verbatim.
		{"1.0.0-alpha", "1.0.0-alpha.1", -1},
		{"1.0.0-alpha.1", "1.0.0-alpha.beta", -1},
		{"1.0.0-alpha.beta", "1.0.0-beta", -1},
		{"1.0.0-beta", "1.0.0-beta.2", -1},
		{"1.0.0-beta.2", "1.0.0-beta.11", -1},
		{"1.0.0-beta.11", "1.0.0-rc.1", -1},
		{"1.0.0-rc.1", "1.0.0", -1},
	}
	for _, c := range cases {
		a, err := Parse(c.a)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.a, err)
		}
		b, err := Parse(c.b)
		if err != nil {
			t.Fatalf("Parse(%q): %v", c.b, err)
		}
		if got := Compare(a, b); got != c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d", c.a, c.b, got, c.want)
		}
		if got := Compare(b, a); got != -c.want {
			t.Errorf("Compare(%s, %s) = %d, want %d — not antisymmetric", c.b, c.a, got, -c.want)
		}
	}
}

// TestLessOnUnparseable pins the documented direction: an unknown installed
// version looks older than anything shipped, so migrations run rather than
// being silently skipped.
func TestLessOnUnparseable(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"", "0.1.0", true},
		{"garbage", "0.1.0", true},
		{"0.1.0", "", false},
		{"", "", false},
		{"garbage", "nonsense", false},
	}
	for _, c := range cases {
		if got := Less(c.a, c.b); got != c.want {
			t.Errorf("Less(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}

func TestString(t *testing.T) {
	for _, s := range []string{"1.2.3", "1.0.0-rc.1"} {
		v, err := Parse(s)
		if err != nil {
			t.Fatal(err)
		}
		if v.String() != s {
			t.Errorf("Parse(%q).String() = %q", s, v.String())
		}
	}
	// Build metadata is accepted and dropped, as the spec allows.
	v, _ := Parse("1.2.3+build.5")
	if v.String() != "1.2.3" {
		t.Errorf("build metadata survived: %q", v.String())
	}
}
