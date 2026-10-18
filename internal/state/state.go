// Package state reads and writes .trunkline/state.yaml — what the project has
// installed and at which versions. It is the file `update` diffs against and
// `doctor` reports from, so it records a hash per installed file: a file the
// user edited locally must never be overwritten silently.
package state

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/vseminoy/trunkline/internal/atomicfile"
)

// Dir is the vendored pipeline root inside a project.
const Dir = ".trunkline"

// File is the state file, relative to the project root.
const File = Dir + "/state.yaml"

type State struct {
	// Core is the semver of the pipeline core installed in this project.
	Core string `yaml:"core"`
	// CLI is the version of the binary that last wrote this file. Equal to
	// Core today; kept separate so the two can diverge later without a
	// migration.
	CLI string `yaml:"cli"`
	// Profiles maps an installed profile name to its manifest version.
	Profiles map[string]string `yaml:"profiles"`
	// Language is the localization installed in this project. It is recorded
	// here, and not read back from the flags block, because `update` must not
	// depend on prose: a hand-written block, or one long line that defeats a
	// parser, would read as "no language" and revert a localized project to
	// English — the one thing update must never do.
	Language string `yaml:"language,omitempty"`
	// Files maps a project-relative path to the sha256 of the content
	// trunkline wrote there. Used to detect local edits before overwriting.
	Files map[string]string `yaml:"files"`
	// Migrations lists the ids of migrations already applied.
	Migrations []string `yaml:"migrations,omitempty"`
	UpdatedAt  string   `yaml:"updated_at"`
}

func New() *State {
	return &State{Profiles: map[string]string{}, Files: map[string]string{}}
}

// Load reads the state of the project rooted at dir. A missing file is not an
// error: it means "not installed", reported by Exists.
func Load(dir string) (*State, error) {
	b, err := os.ReadFile(filepath.Join(dir, File))
	if os.IsNotExist(err) {
		return New(), nil
	}
	if err != nil {
		return nil, err
	}
	s := New()
	if err := yaml.Unmarshal(b, s); err != nil {
		return nil, fmt.Errorf("parse %s: %w", File, err)
	}
	if s.Profiles == nil {
		s.Profiles = map[string]string{}
	}
	if s.Files == nil {
		s.Files = map[string]string{}
	}
	// This file is committed on purpose, so it arrives with the checkout rather
	// than being produced locally. A path in it that leaves the project is not a
	// typo to tolerate: `update` would follow it out of the project and delete
	// what it found there. Refuse the whole file and say which entry.
	for rel := range s.Files {
		if !ValidRel(rel) {
			return nil, fmt.Errorf("%s lists %q, which is not a path inside the "+
				"project; refusing to act on it", File, rel)
		}
	}
	return s, nil
}

// ValidRel reports whether rel can name a file of this project: a clean,
// relative, slash-separated path that stays inside the root. Everything
// trunkline writes or deletes goes through this.
func ValidRel(rel string) bool {
	if rel == "" || filepath.IsAbs(rel) || strings.ContainsRune(rel, '\\') {
		return false
	}
	if strings.ContainsRune(rel, 0) {
		return false
	}
	clean := path.Clean(rel)
	if clean == "." || clean == ".." || strings.HasPrefix(clean, "../") {
		return false
	}
	return true
}

func Exists(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, File))
	return err == nil
}

func (s *State) Save(dir string) error {
	s.UpdatedAt = time.Now().UTC().Format(time.RFC3339)
	b, err := yaml.Marshal(s)
	if err != nil {
		return err
	}
	header := "# Written by trunkline. Records what is installed and at which\n" +
		"# version; `trunkline update` diffs against it. Commit this file.\n"
	// Atomically: a truncated state file is worse than an outdated one. Half a
	// file list means half the local edits lose their protection, and `update`
	// cannot tell the difference.
	return atomicfile.Write(filepath.Join(dir, File), append([]byte(header), b...), 0o644)
}

func (s *State) Applied(id string) bool {
	return slices.Contains(s.Migrations, id)
}

func (s *State) MarkApplied(id string) {
	if !s.Applied(id) {
		s.Migrations = append(s.Migrations, id)
	}
}

func Hash(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// LocallyModified reports whether the file at rel differs from what trunkline
// recorded writing there.
//
// The two boundary cases point in opposite directions, and both are deliberate.
// A file that is gone is not modified — writing it back is the repair, not an
// overwrite. A file that cannot be read counts as modified: an unreadable file
// is one we cannot compare, and the whole mechanism exists to avoid destroying
// what we have not looked at.
//
// A path we have no record of is not answered here at all — see Tracked. That
// question is "is this ours", not "has it changed", and conflating the two is
// how a file present on disk but absent from the state got overwritten as
// though trunkline had written it.
func (s *State) LocallyModified(dir, rel string) bool {
	want, ok := s.Files[rel]
	if !ok {
		return false
	}
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if os.IsNotExist(err) {
		return false
	}
	if err != nil {
		return true
	}
	return Hash(b) != want
}

// Tracked reports whether trunkline recorded writing this path.
func (s *State) Tracked(rel string) bool {
	_, ok := s.Files[rel]
	return ok
}
