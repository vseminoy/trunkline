// Package profile reads a stack profile manifest. Only the slots the CLI acts
// on are typed; everything else in profile.yaml is the pipeline's business and
// is deliberately left untouched.
//
// Paths inside the manifest (guidelines, step overlays, hooks) are relative to
// the profile's own directory, and this package is the single place that
// resolves them — see Path. A profile is a movable pack: copying the directory
// under a new name and editing its contents has to be enough, so a path that
// names the profile it lives in is a defect, not a style choice. It survives
// the linter (the original profile's file does exist) and surfaces later as the
// copy quietly reading someone else's guidelines.
package profile

import (
	"fmt"
	"io/fs"
	"path"
	"strings"

	"gopkg.in/yaml.v3"
)

type Manifest struct {
	// dir is where the manifest was loaded from, relative to the root of the
	// filesystem it came from. Not a manifest field: it is how the profile
	// knows what its own paths are relative to.
	dir string

	Name        string `yaml:"name"`
	Version     string `yaml:"version"`
	Language    string `yaml:"language"`
	Description string `yaml:"description"`

	Commands             map[string]string `yaml:"commands"`
	RequiredProjectFiles []RequiredFile    `yaml:"required_project_files"`

	Guidelines struct {
		Root  string            `yaml:"root"`
		Index string            `yaml:"index"`
		Roles map[string]string `yaml:"roles"`
	} `yaml:"guidelines"`

	StepOverlays map[string]string `yaml:"step_overlays"`

	// Tasks is the analytic track's tracker. Absent or null means the profile
	// runs the developer track only — the analytic track is not available at
	// all, since format_spec is what a task's format is and dir is where one
	// goes. A profile that declares Tasks but no Publisher is the other case:
	// there the analyst works as usual and stops at the files.
	//
	// Two anchors on purpose: FormatSpec and Publisher.Setup are documents of
	// the profile and travel with it into a project, so they are relative to the
	// profile directory. Dir and AnalysisDir are the project's own data — the
	// tasks describe that project's code and are committed beside it — so
	// they are relative to the project root.
	Tasks *struct {
		FormatSpec  string `yaml:"format_spec"`
		Dir         string `yaml:"dir"`
		AnalysisDir string `yaml:"analysis_dir"`
		Publisher   struct {
			Name  string `yaml:"name"`
			Setup string `yaml:"setup"`
			// Contract is the publisher's own document: what its code
			// requires of the files it consumes. Anchored at the pipeline
			// root (publishers/…), not at the profile, because the tool owns
			// it and several profiles may declare the same publisher.
			Contract   string            `yaml:"contract"`
			Operations map[string]string `yaml:"operations"`
		} `yaml:"publisher"`
	} `yaml:"tasks"`

	Hooks struct {
		DocCacheTargets []string `yaml:"doc_cache_targets"`
		PreCommitGate   string   `yaml:"pre_commit_gate"`
	} `yaml:"hooks"`
}

type RequiredFile struct {
	Path string `yaml:"path"`
	Why  string `yaml:"why"`
}

// Load reads <dir>/profile.yaml out of fsys. Dir is remembered so the manifest
// can resolve its own paths; see Path.
func Load(fsys fs.FS, dir string) (Manifest, error) {
	var m Manifest
	b, err := fs.ReadFile(fsys, path.Join(dir, "profile.yaml"))
	if err != nil {
		return m, fmt.Errorf("read profile manifest: %w", err)
	}
	if err := yaml.Unmarshal(b, &m); err != nil {
		return m, fmt.Errorf("parse profile manifest: %w", err)
	}
	if m.Name == "" {
		return m, fmt.Errorf("profile manifest has no name")
	}
	if m.Version == "" {
		return m, fmt.Errorf("profile %q has no version", m.Name)
	}
	// The directory name is what every caller addresses the profile by — the
	// flags block, `profile add`, the vendored path. A manifest whose name
	// disagrees with it would have doctor checking one profile while the agent
	// reads another.
	if base := path.Base(dir); base != "." && base != m.Name {
		return m, fmt.Errorf("profile in %s/ declares name %q — the directory name and "+
			"the manifest name must match", dir, m.Name)
	}
	m.dir = dir
	if err := m.validatePaths(); err != nil {
		return m, err
	}
	return m, nil
}

// Path resolves one manifest path against the profile directory. Every consumer
// goes through here, so a slot has exactly one meaning: before this existed the
// CLI resolved the same slot three different ways — one stripped a prefix, one
// ignored the slot and rebuilt the path from a hardcoded directory name, and the
// linter joined it to the repository root.
//
// The returned path is relative to the root the profile was loaded from (the
// repository root here, .trunkline/ in an installed project).
func (m Manifest) Path(rel string) string {
	if rel == "" {
		return ""
	}
	return path.Join(m.dir, rel)
}

// RolePath is Path for a guideline role: the file names in guidelines.roles are
// relative to guidelines.root, not to the profile directory.
func (m Manifest) RolePath(role string) string {
	file, ok := m.Guidelines.Roles[role]
	if !ok {
		return ""
	}
	return m.Path(path.Join(m.Guidelines.Root, file))
}

// validatePaths rejects manifest paths that cannot mean what they say: absolute
// ones, ones climbing out of the profile, and ones that still carry the old
// repository-relative `profiles/<name>/` prefix. The last is worth its own
// message: it used to be the documented form, and it fails in a way that looks
// like a missing file rather than a wrong anchor.
func (m Manifest) validatePaths() error {
	check := func(slot, val string) error {
		switch {
		case val == "":
			return nil
		case path.IsAbs(val):
			return fmt.Errorf("%s: %q is absolute — manifest paths are relative to the "+
				"profile directory", slot, val)
		case strings.HasPrefix(val, "profiles/"):
			return fmt.Errorf("%s: %q is anchored to the repository — manifest paths are "+
				"relative to the profile directory, so drop the %q prefix",
				slot, val, "profiles/"+m.Name+"/")
		case val == ".." || strings.HasPrefix(val, "../") ||
			strings.Contains(val, "/../") || strings.HasSuffix(val, "/.."):
			return fmt.Errorf("%s: %q leaves the profile directory", slot, val)
		}
		return nil
	}

	if err := check("guidelines.root", m.Guidelines.Root); err != nil {
		return err
	}
	if err := check("guidelines.index", m.Guidelines.Index); err != nil {
		return err
	}
	for role, file := range m.Guidelines.Roles {
		if err := check("guidelines.roles."+role, file); err != nil {
			return err
		}
	}
	for step, file := range m.StepOverlays {
		if err := check("step_overlays."+step, file); err != nil {
			return err
		}
	}
	if err := check("hooks.pre_commit_gate", m.Hooks.PreCommitGate); err != nil {
		return err
	}
	if m.Tasks != nil {
		if err := check("tasks.format_spec", m.Tasks.FormatSpec); err != nil {
			return err
		}
		if err := check("tasks.publisher.setup", m.Tasks.Publisher.Setup); err != nil {
			return err
		}
		// The contract is the one manifest path anchored at the pipeline root:
		// it belongs to the tool, which no profile owns.
		if c := m.Tasks.Publisher.Contract; c != "" && !strings.HasPrefix(c, "publishers/") {
			return fmt.Errorf("tasks.publisher.contract: %q must live under publishers/ — "+
				"a publisher's contract belongs to the tool, not to a profile", c)
		}
		// tasks.dir and tasks.analysis_dir name the project's data, not the
		// profile's, so they must not reach into the pipeline tree. They may
		// leave the project: an analysis kept in its own repository — a sibling
		// checkout or a submodule — is the arrangement that keeps the churn of
		// the analyst's state out of the code history, and the `knowledge` flag
		// already allows the same for a neighbouring service's base. What such a
		// path costs is that git cannot vouch for it, so `trunkline doctor`
		// checks that it exists and is not empty; absolute paths stay out because
		// those cannot travel between machines at all.
		for slot, val := range map[string]string{
			"tasks.dir": m.Tasks.Dir, "tasks.analysis_dir": m.Tasks.AnalysisDir,
		} {
			switch {
			case val == "":
			case path.IsAbs(val):
				return fmt.Errorf("%s: %q is absolute — it is relative to the project root", slot, val)
			case strings.HasPrefix(val, "core/"), strings.HasPrefix(val, "profiles/"),
				strings.HasPrefix(val, "agent-skills/"):
				return fmt.Errorf("%s: %q points into the pipeline — this slot names the "+
					"project's own data, not the tool's", slot, val)
			}
		}
	}
	return nil
}

// List returns the profile names available under the "profiles/" root of fsys.
func List(fsys fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(fsys, "profiles")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		if _, err := fs.Stat(fsys, path.Join("profiles", e.Name(), "profile.yaml")); err != nil {
			continue
		}
		names = append(names, e.Name())
	}
	return names, nil
}
