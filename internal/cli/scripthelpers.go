package cli

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// execTimeout is context.WithTimeout spelled out once so every command that
// shells out to git gets the same bounded wait.
func execTimeout(d time.Duration) (context.Context, context.CancelFunc) {
	return context.WithTimeout(context.Background(), d)
}

// repeatedFlag collects a flag.Var-backed flag that may be passed more than
// once (--column, --partial, --na, --guideline), in the order given.
type repeatedFlag []string

func (r *repeatedFlag) String() string { return strings.Join(*r, ",") }
func (r *repeatedFlag) Set(v string) error {
	*r = append(*r, v)
	return nil
}

// realpath resolves symlinks the way Python's os.path.realpath does, falling
// back to the absolute path when the file does not exist or cannot be
// resolved (a caller that only needs a stable key for deduplication still
// gets one).
func realpath(p string) string {
	abs, err := filepath.Abs(p)
	if err != nil {
		return p
	}
	resolved, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return abs
	}
	return resolved
}

// readTextLenient reads a file as UTF-8, replacing invalid sequences with the
// replacement character instead of failing — Python's errors="replace".
func readTextLenient(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if utf8.Valid(b) {
		return string(b), nil
	}
	var sb strings.Builder
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			sb.WriteRune('�')
			b = b[1:]
			continue
		}
		sb.WriteRune(r)
		b = b[size:]
	}
	return sb.String(), nil
}
