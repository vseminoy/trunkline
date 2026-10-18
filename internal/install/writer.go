package install

import (
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"github.com/vseminoy/trunkline/internal/atomicfile"
	"github.com/vseminoy/trunkline/internal/state"
)

// Status is what happened to one file.
type Status string

const (
	Created   Status = "created"
	Updated   Status = "updated"
	Unchanged Status = "unchanged"
	// Kept means the file differs from what we wrote last time, so the user
	// edited it and we left it alone.
	Kept Status = "kept (locally modified)"
	// Foreign means the file was already on disk and trunkline has no record of
	// putting it there. Distinct from Kept on purpose: "you edited ours" and
	// "this was never ours" call for different answers from the user, and the
	// second is the one that used to be silently overwritten.
	Foreign Status = "kept (not ours)"
)

// Writer applies file changes to a project, recording what it wrote in the
// state so a later update can tell its own output from the user's edits.
type Writer struct {
	Dir    string
	State  *state.State
	DryRun bool
	Force  bool

	changes []change
}

type change struct {
	Rel    string
	Status Status
}

func NewWriter(dir string, st *state.State) *Writer {
	return &Writer{Dir: dir, State: st}
}

// Write puts content at the project-relative path rel.
//
// Two kinds of file are left alone unless Force is set: one we wrote and the
// user has since edited (Kept), and one that is simply there with no record of
// us writing it (Foreign). The second case is the important one — a project
// whose state.yaml was never written or was deleted has a file on disk for every
// path we are about to write, and treating "no record" as "not ours to protect"
// destroys exactly what the hash mechanism exists to preserve.
func (w *Writer) Write(rel string, content []byte, mode os.FileMode) (Status, error) {
	abs, err := safeJoin(w.Dir, rel)
	if err != nil {
		return "", fmt.Errorf("refusing to write: %w", err)
	}
	sum := state.Hash(content)

	existing, err := os.ReadFile(abs)
	switch {
	case err != nil && !os.IsNotExist(err):
		// Unreadable is not absent. Treating it as absent is how a file we could
		// not even look at got overwritten.
		return "", fmt.Errorf("read %s: %w", rel, err)

	case err == nil && state.Hash(existing) == sum:
		w.State.Files[rel] = sum
		// The content matches, so nothing is written — which is precisely when a
		// wrong mode would go unnoticed for good. See atomicfile.EnsureExecutable.
		if !w.DryRun {
			fixed, err := atomicfile.EnsureExecutable(abs, mode)
			if err != nil {
				return "", err
			}
			if fixed {
				w.record(rel, Updated)
				return Updated, nil
			}
		}
		w.record(rel, Unchanged)
		return Unchanged, nil

	case err == nil && !w.Force && !w.State.Tracked(rel):
		w.record(rel, Foreign)
		return Foreign, nil

	case err == nil && !w.Force && state.Hash(existing) != w.State.Files[rel]:
		w.record(rel, Kept)
		return Kept, nil
	}

	status := Created
	if err == nil {
		status = Updated
	}
	if !w.DryRun {
		if err := atomicfile.Write(abs, content, mode); err != nil {
			return status, err
		}
	}
	w.State.Files[rel] = sum
	w.record(rel, status)
	return status, nil
}

// CopyTree copies srcRoot out of fsys into the project at dstRoot, preserving
// the tree shape. Executable bits are set for .sh files, which is the only
// executable content the pipeline ships.
//
// skip lists source path prefixes to leave behind — used for subtrees that are
// build material rather than payload, such as the localizations.
func (w *Writer) CopyTree(fsys fs.FS, srcRoot, dstRoot string, skip ...string) error {
	return fs.WalkDir(fsys, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if skipped(p, skip) {
			return nil
		}
		b, err := fs.ReadFile(fsys, p)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.FromSlash(srcRoot), filepath.FromSlash(p))
		if err != nil {
			return err
		}
		mode := os.FileMode(0o644)
		if strings.HasSuffix(p, ".sh") {
			mode = 0o755
		}
		_, err = w.Write(path.Join(dstRoot, filepath.ToSlash(rel)), b, mode)
		return err
	})
}

// Prune deletes files we previously wrote under prefix that are no longer part
// of what we ship — a step removed from the core must not linger in a project.
func (w *Writer) Prune(prefix string, shipped map[string]bool) error {
	var stale []string
	for rel := range w.State.Files {
		if strings.HasPrefix(rel, prefix) && !shipped[rel] {
			stale = append(stale, rel)
		}
	}
	sort.Strings(stale)
	for _, rel := range stale {
		abs, err := safeJoin(w.Dir, rel)
		if err != nil {
			return fmt.Errorf("refusing to prune an entry of %s: %w", state.File, err)
		}
		if w.State.LocallyModified(w.Dir, rel) && !w.Force {
			w.record(rel, Kept)
			continue
		}
		if !w.DryRun {
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
		delete(w.State.Files, rel)
		w.record(rel, "removed")
	}
	return nil
}

// skipped reports whether a source path falls under one of the skip prefixes.
func skipped(p string, skip []string) bool {
	for _, s := range skip {
		if strings.HasPrefix(p, s) {
			return true
		}
	}
	return false
}

// Shipped lists the project-relative destinations CopyTree would produce, so
// Prune can tell "gone from the release" from "never ours". It must be given the
// same skip list as CopyTree, or Prune deletes what CopyTree deliberately did
// not write.
func Shipped(fsys fs.FS, srcRoot, dstRoot string, skip ...string) (map[string]bool, error) {
	out := map[string]bool{}
	err := fs.WalkDir(fsys, srcRoot, func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		if skipped(p, skip) {
			return nil
		}
		rel, relErr := filepath.Rel(filepath.FromSlash(srcRoot), filepath.FromSlash(p))
		if relErr != nil {
			return relErr
		}
		out[path.Join(dstRoot, filepath.ToSlash(rel))] = true
		return nil
	})
	return out, err
}

func (w *Writer) record(rel string, s Status) {
	w.changes = append(w.changes, change{Rel: rel, Status: s})
}

// Summary folds the per-file changes into one line per status, plus the full
// list of files that were left alone — those are the ones where the project on
// disk and the payload now differ, and a count alone would not say which.
func (w *Writer) Summary() string {
	counts := map[Status]int{}
	var kept []string
	for _, c := range w.changes {
		counts[c.Status]++
		if c.Status == Kept || c.Status == Foreign {
			kept = append(kept, string(c.Status)+" "+c.Rel)
		}
	}
	var b strings.Builder
	for _, s := range []Status{Created, Updated, Unchanged, "removed", Kept, Foreign} {
		if n := counts[s]; n > 0 {
			fmt.Fprintf(&b, "  %-24s %d\n", string(s)+":", n)
		}
	}
	sort.Strings(kept)
	for _, k := range kept {
		fmt.Fprintf(&b, "    %s\n", k)
	}
	if counts[Foreign] > 0 {
		fmt.Fprintf(&b, "  %d file(s) were already there and are not recorded as ours; "+
			"rerun with --force to replace them.\n", counts[Foreign])
	}
	return b.String()
}
