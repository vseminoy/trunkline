// Package semver implements just enough of semver 2.0 for the CLI: parsing
// MAJOR.MINOR.PATCH with an optional pre-release, and ordering two versions.
// Build metadata is accepted and ignored, as the spec requires.
package semver

import (
	"fmt"
	"strconv"
	"strings"
)

type Version struct {
	Major, Minor, Patch int
	Pre                 string
}

func Parse(s string) (Version, error) {
	var v Version
	s = strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(s), "v"))
	if s == "" {
		return v, fmt.Errorf("empty version")
	}
	if i := strings.IndexByte(s, '+'); i >= 0 {
		s = s[:i]
	}
	if i := strings.IndexByte(s, '-'); i >= 0 {
		v.Pre, s = s[i+1:], s[:i]
	}
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return v, fmt.Errorf("not a MAJOR.MINOR.PATCH version: %q", s)
	}
	dst := []*int{&v.Major, &v.Minor, &v.Patch}
	for i, p := range parts {
		// Digits only, and no leading zero: `01.2.3` is not a semver, and
		// accepting it would make two spellings of the same version compare
		// equal in one place and differ as strings in another.
		if !numeric(p) || (len(p) > 1 && p[0] == '0') {
			return v, fmt.Errorf("not a MAJOR.MINOR.PATCH version: %q", s)
		}
		n, err := strconv.Atoi(p)
		if err != nil {
			return v, fmt.Errorf("not a MAJOR.MINOR.PATCH version: %q", s)
		}
		*dst[i] = n
	}
	return v, nil
}

// numeric reports whether s is a non-empty run of ASCII digits. strconv.Atoi
// alone is not enough: it accepts a sign, and "+1" is not a version component.
func numeric(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

func (v Version) String() string {
	s := fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
	if v.Pre != "" {
		s += "-" + v.Pre
	}
	return s
}

// Compare returns -1, 0 or 1. A pre-release sorts before its release.
func Compare(a, b Version) int {
	for _, p := range [][2]int{{a.Major, b.Major}, {a.Minor, b.Minor}, {a.Patch, b.Patch}} {
		if p[0] != p[1] {
			return sign(p[0] - p[1])
		}
	}
	switch {
	case a.Pre == b.Pre:
		return 0
	case a.Pre == "":
		return 1
	case b.Pre == "":
		return -1
	}
	return comparePre(a.Pre, b.Pre)
}

// comparePre orders two pre-release strings the way semver 2.0 requires:
// identifier by identifier, numeric ones compared as numbers, a numeric
// identifier below an alphanumeric one, and a shorter prefix below a longer one.
//
// A plain string comparison gets this wrong in the one place it matters:
// `rc.10` would sort before `rc.9`, which is exactly the point at which a
// release series stops being a toy. migrate.Pending decides both of its bounds
// with Less, so the wrong order there means a migration silently not running.
func comparePre(a, b string) int {
	as, bs := strings.Split(a, "."), strings.Split(b, ".")
	for i := 0; i < len(as) && i < len(bs); i++ {
		x, y := as[i], bs[i]
		nx, ny := numeric(x), numeric(y)
		switch {
		case nx && ny:
			ix, _ := strconv.Atoi(x)
			iy, _ := strconv.Atoi(y)
			if ix != iy {
				return sign(ix - iy)
			}
		case nx:
			return -1
		case ny:
			return 1
		default:
			if c := strings.Compare(x, y); c != 0 {
				return c
			}
		}
	}
	return sign(len(as) - len(bs))
}

// Less reports whether a < b. Unparseable versions sort first, which makes an
// unknown installed version look older than anything shipped — the safe
// direction: migrations run rather than being silently skipped.
func Less(a, b string) bool {
	pa, errA := Parse(a)
	pb, errB := Parse(b)
	switch {
	case errA != nil && errB != nil:
		return false
	case errA != nil:
		return true
	case errB != nil:
		return false
	}
	return Compare(pa, pb) < 0
}

func sign(n int) int {
	switch {
	case n < 0:
		return -1
	case n > 0:
		return 1
	}
	return 0
}
