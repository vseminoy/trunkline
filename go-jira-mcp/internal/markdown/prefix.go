package markdown

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// prefixRe matches the leading alphanumeric code of a task file name, e.g. the
// "C2" in "C2-fix-excel-export.md": one or more letters followed by digits.
var prefixRe = regexp.MustCompile(`^([A-Za-z]+)(\d+)`)

// Prefix is the lexicographic-then-numeric ordering key parsed from a task file
// name (letter group + number), e.g. {Letter:"C", Num:2}.
type Prefix struct {
	Letter string // upper-cased letter group, e.g. "C"
	Num    int    // numeric part, compared as a number
	Raw    string // the matched prefix as it appeared, e.g. "C2"
}

// ParsePrefix extracts the ordering prefix from a file name. It returns an error
// if the name does not start with a letter+number code (TZ §13: invalid prefix
// is a task error and is skipped with a report message).
func ParsePrefix(fileName string) (Prefix, error) {
	m := prefixRe.FindStringSubmatch(fileName)
	if m == nil {
		return Prefix{}, fmt.Errorf("file name %q has no letter+number prefix (expected e.g. C2-...)", fileName)
	}
	num, err := strconv.Atoi(m[2])
	if err != nil {
		return Prefix{}, fmt.Errorf("file name %q has invalid numeric prefix: %w", fileName, err)
	}
	return Prefix{
		Letter: strings.ToUpper(m[1]),
		Num:    num,
		Raw:    m[1] + m[2],
	}, nil
}

// Less reports whether p sorts before other: by letter group first, then by the
// numeric part. Used to order task creation so blockers are created first.
func (p Prefix) Less(other Prefix) bool {
	if p.Letter != other.Letter {
		return p.Letter < other.Letter
	}
	return p.Num < other.Num
}
