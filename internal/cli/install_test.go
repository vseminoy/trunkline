package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vseminoy/trunkline/internal/install"
	"github.com/vseminoy/trunkline/internal/state"
)

// run executes one command against a temporary project and returns its output.
func run(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	var out, errOut bytes.Buffer
	env := Env{Out: &out, Err: &errOut, Dir: dir}
	var err error
	switch args[0] {
	case "init":
		err = runInit(env, args[1:])
	case "update":
		err = runUpdate(env, args[1:])
	case "doctor":
		err = runDoctor(env, args[1:])
	default:
		t.Fatalf("unknown command %q", args[0])
	}
	return out.String() + errOut.String(), err
}

// aProfile is any profile that ships, so the tests do not pin one by name.
func aProfile(t *testing.T) string {
	t.Helper()
	name, err := resolveProfile("")
	if err == nil {
		return name
	}
	// Several ship: take the first alphabetically, whatever it is.
	msg := err.Error()
	i := strings.Index(msg, "available: ")
	if i < 0 {
		t.Fatalf("cannot find a profile to test with: %v", err)
	}
	return strings.Split(msg[i+len("available: "):], ", ")[0]
}

func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	abs := filepath.Join(dir, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(abs, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestReadFlags(t *testing.T) {
	t.Run("parses the block and stops at the next heading", func(t *testing.T) {
		dir := t.TempDir()
		writeFile(t, dir, install.FlagsFile, `# Code standards

## Pipeline flags

- profile: demo-go   # the stack
- language: ru
- knowledge: none
# - rps: 2000
# - host: cursor

## Something else

- profile: not-this-one
`)
		flags, err := readFlags(dir)
		if err != nil {
			t.Fatal(err)
		}
		want := map[string]string{"profile": "demo-go", "language": "ru", "knowledge": "none"}
		for k, v := range want {
			if flags[k] != v {
				t.Errorf("flags[%q] = %q, want %q", k, flags[k], v)
			}
		}
		// A commented-out flag is not set, and must not end the block either.
		if _, ok := flags["rps"]; ok {
			t.Error("a commented-out flag was read as set")
		}
		if _, ok := flags["host"]; ok {
			t.Error("a commented-out flag was read as set")
		}
	})

	t.Run("a missing file is no flags, not an error", func(t *testing.T) {
		flags, err := readFlags(t.TempDir())
		if err != nil || len(flags) != 0 {
			t.Errorf("readFlags = %v, %v; want empty and no error", flags, err)
		}
	})

	t.Run("a line longer than a scanner buffer is still parsed", func(t *testing.T) {
		// The failure this pins: bufio.Scanner stops at 64 KiB, and one minified
		// JSON blob or base64 payload in AGENTS.md was enough to truncate the
		// parse. `language` then read as unset, which means English, and update
		// reverted a localized project.
		dir := t.TempDir()
		writeFile(t, dir, install.FlagsFile, "# Code standards\n\n"+
			strings.Repeat("x", 200*1024)+"\n\n## Pipeline flags\n\n- language: ru\n")
		flags, err := readFlags(dir)
		if err != nil {
			t.Fatal(err)
		}
		if flags["language"] != "ru" {
			t.Errorf("language = %q, want ru", flags["language"])
		}
	})

	t.Run("an unreadable file is an error, not an empty set", func(t *testing.T) {
		if os.Geteuid() == 0 {
			t.Skip("root reads everything")
		}
		dir := t.TempDir()
		writeFile(t, dir, install.FlagsFile, "## Pipeline flags\n\n- language: ru\n")
		if err := os.Chmod(filepath.Join(dir, install.FlagsFile), 0o000); err != nil {
			t.Fatal(err)
		}
		if _, err := readFlags(dir); err == nil {
			t.Error("an unreadable AGENTS.md read as an empty set of flags; " +
				"every flag then falls back to a default nobody chose")
		}
	})
}

func TestResolveLanguage(t *testing.T) {
	if _, err := resolveLanguage("en"); err != nil {
		t.Errorf("en: %v", err)
	}
	if got, _ := resolveLanguage(""); got != install.DefaultLanguage {
		t.Errorf("empty = %q, want %q", got, install.DefaultLanguage)
	}
	// A path that happens to resolve is not a language. `..` named the l10n
	// directory itself, passed, and landed in the flags block.
	for _, bad := range []string{"..", ".", "../core", "nosuchlang", "ru/"} {
		if got, err := resolveLanguage(bad); err == nil {
			t.Errorf("resolveLanguage(%q) = %q, want it refused", bad, got)
		}
	}
}

// TestActiveLanguagePrefersState is the revert bug: EnsureFlags never rewrites
// an existing block, so a project whose AGENTS.md already had one never got
// `language: ru` written into it — and update read the absence as English.
func TestActiveLanguagePrefersState(t *testing.T) {
	langs, _ := languages()
	if len(langs) < 2 {
		t.Skip("only English ships in this build")
	}
	other := langs[0]
	if other == install.DefaultLanguage {
		other = langs[1]
	}

	dir := t.TempDir()
	writeFile(t, dir, install.FlagsFile, "# Code standards\n\n## Pipeline flags\n\n- profile: p\n")
	st := state.New()
	st.Language = other

	got, err := activeLanguage(dir, st)
	if err != nil {
		t.Fatal(err)
	}
	if got != other {
		t.Errorf("activeLanguage = %q, want %q — a localized project must not revert to English", got, other)
	}
}

// TestActiveLanguageFallsBackToFlags: a project installed before the state
// recorded the language still reads it from the block.
func TestActiveLanguageFallsBackToFlags(t *testing.T) {
	langs, _ := languages()
	if len(langs) < 2 {
		t.Skip("only English ships in this build")
	}
	other := langs[0]
	if other == install.DefaultLanguage {
		other = langs[1]
	}

	dir := t.TempDir()
	writeFile(t, dir, install.FlagsFile,
		"## Pipeline flags\n\n- profile: p\n- language: "+other+"\n")
	got, err := activeLanguage(dir, state.New())
	if err != nil {
		t.Fatal(err)
	}
	if got != other {
		t.Errorf("activeLanguage = %q, want %q", got, other)
	}
}

// TestL10nDriftMatchesTheLint: whatever scripts/lint-l10n.py would report in
// this tree, the binary reports the same, because it reads the same lock file.
// The assertion is only that the check runs and is self-consistent — a tree
// with a drifted translation is a legitimate state to build from, and the
// point is that it is announced rather than shipped in silence.
func TestL10nDrift(t *testing.T) {
	if _, err := l10nDrift(install.DefaultLanguage); err != nil {
		t.Errorf("English must never be drifted from itself: %v", err)
	}
	if _, err := l10nDrift("nosuchlang"); err == nil {
		t.Error("a language with no lock file reported no drift; " +
			"absent evidence is not evidence of currency")
	}
	langs, _ := languages()
	for _, lang := range langs {
		if lang == install.DefaultLanguage {
			continue
		}
		drift, err := l10nDrift(lang)
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if len(drift) > 0 && l10nDriftMessage(lang, drift) == "" {
			t.Errorf("%s: drift found but nothing said about it", lang)
		}
	}
}

// TestInitIsIdempotent is the property the whole state file exists to provide:
// installing twice changes nothing the second time.
func TestInitIsIdempotent(t *testing.T) {
	dir := t.TempDir()
	name := aProfile(t)

	if _, err := run(t, dir, "init", "--profile", name, "--gate=none"); err != nil {
		t.Fatalf("first init: %v", err)
	}
	out, err := run(t, dir, "init", "--profile", name, "--gate=none")
	if err != nil {
		t.Fatalf("second init: %v", err)
	}
	for _, unwanted := range []string{"created:", "updated:", "removed:", "kept ("} {
		if strings.Contains(out, unwanted) {
			t.Errorf("a second install reported %q:\n%s", unwanted, out)
		}
	}
}

// TestInitValidatesBeforeWriting is the half-installed-project bug: a bad
// --gate used to be caught after the payload was already on disk, leaving a
// project with no state.yaml that `update` then refused to touch.
func TestInitValidatesBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init", "--profile", aProfile(t), "--gate=nonsense"); err == nil {
		t.Fatal("a bad --gate was accepted")
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("the run failed but left %v behind", names)
	}
}

// TestInitRecordsStateOnFailure: once files exist, the record of them must
// survive whatever happens next, or the next run cannot tell them from the
// user's own files.
func TestInitRecordsStateOnFailure(t *testing.T) {
	dir := t.TempDir()
	// A directory where AGENTS.md must go makes the flags step fail after the
	// payload has been written.
	if err := os.MkdirAll(filepath.Join(dir, install.AgentsFile), 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := run(t, dir, "init", "--profile", aProfile(t), "--gate=none"); err == nil {
		t.Skip("this build writes AGENTS.md without failing on a directory")
	}
	if !state.Exists(dir) {
		t.Fatalf("%s was not written, so nothing on disk is recorded as ours", state.File)
	}
	st, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(st.Files) == 0 {
		t.Error("the state records no files although the payload was written")
	}
}

// TestUpdateKeepsLocalEdits: the promise on the tin — an edited vendored file
// is kept and named, never overwritten silently.
func TestUpdateKeepsLocalEdits(t *testing.T) {
	dir := t.TempDir()
	name := aProfile(t)
	if _, err := run(t, dir, "init", "--profile", name, "--gate=none"); err != nil {
		t.Fatal(err)
	}

	st, err := state.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	var victim string
	for rel := range st.Files {
		if strings.HasPrefix(rel, install.CoreDir+"/") && strings.HasSuffix(rel, ".md") {
			if victim == "" || rel < victim {
				victim = rel
			}
		}
	}
	if victim == "" {
		t.Fatal("no vendored core file to edit")
	}
	writeFile(t, dir, victim, "MY OWN VERSION\n")

	out, err := run(t, dir, "update")
	if err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, dir, victim); got != "MY OWN VERSION\n" {
		t.Errorf("the edit was overwritten: %q", got)
	}
	if !strings.Contains(out, victim) {
		t.Errorf("the kept file was not named in the summary:\n%s", out)
	}
}

func readFile(t *testing.T, dir, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// TestUpdateRefusesWithoutInit keeps the message that tells the user what to do.
func TestUpdateRefusesWithoutInit(t *testing.T) {
	out, err := run(t, t.TempDir(), "update")
	if err == nil {
		t.Fatal("update ran on a project with no install")
	}
	if !strings.Contains(err.Error()+out, "init") {
		t.Errorf("the error does not point at `init`: %v", err)
	}
}

// TestDryRunWritesNothing: --dry-run on a clean project must leave it clean.
func TestInitDryRunWritesNothing(t *testing.T) {
	dir := t.TempDir()
	if _, err := run(t, dir, "init", "--profile", aProfile(t), "--gate=none", "--dry-run"); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Errorf("dry run wrote %d entries", len(entries))
	}
}
