// Package atomicfile writes files the way a tool that installs into somebody
// else's repository has to: all-or-nothing, and never through a symlink.
//
// It exists because os.WriteFile does neither. It truncates first, so a full
// disk or a Ctrl-C halfway through leaves AGENTS.md or .claude/settings.json —
// the user's own files, which trunkline reads whole and rewrites — empty, with
// nothing to restore them from. And it follows symlinks, so a checkout where
// AGENTS.md is a link to ~/.bashrc gets that file rewritten instead.
package atomicfile

import (
	"fmt"
	"os"
	"path/filepath"
)

// Write replaces the file at abs with content: a temporary file in the same
// directory, then a rename over the target.
//
// The symlink check is an Lstat, so it is not proof against a link created
// between the check and the rename, and is not meant to be. What it protects
// against is a repository that is already laid out that way, which is the case
// that actually occurs; racing a local attacker on the filesystem is out of
// scope, and saying so is better than implying otherwise.
//
// The mode is applied with an explicit Chmod rather than left to the temporary
// file's own bits, so an executable script comes out executable whatever the
// umask.
func Write(abs string, content []byte, perm os.FileMode) error {
	if fi, err := os.Lstat(abs); err == nil && fi.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("%s is a symlink; refusing to write through it — "+
			"remove the link, or install into a checkout that does not use one", abs)
	}
	dir := filepath.Dir(abs)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".trunkline-*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer os.Remove(tmp) // a no-op once the rename below has succeeded
	if _, err := f.Write(content); err != nil {
		f.Close()
		return err
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, abs)
}

// EnsureExecutable adds the executable bit to a file that is meant to have one
// and does not. It reports whether it had to.
//
// It is needed because a write is skipped when the content already matches, and
// os.WriteFile applies its mode only when creating the file. Without this, one
// `chmod 644 .claude/hooks/pre-commit-gate.sh` survives every reinstall: git
// skips a non-executable hook, the hook's own `[ -x "$gate" ] || exit 0` exits
// zero, and `init` and `doctor` both go on reporting the gate installed while
// every commit passes on a broken build.
//
// Only the executable bit is restored, and only where the caller asked for one.
// Enforcing the full mode would fight a user who deliberately tightened
// permissions, and would report a change on every run for no gain.
func EnsureExecutable(abs string, perm os.FileMode) (bool, error) {
	if perm&0o111 == 0 {
		return false, nil
	}
	fi, err := os.Lstat(abs)
	if err != nil || !fi.Mode().IsRegular() || fi.Mode()&0o111 != 0 {
		return false, err
	}
	return true, os.Chmod(abs, fi.Mode().Perm()|0o111)
}
