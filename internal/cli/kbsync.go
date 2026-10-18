package cli

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

// kb-sync — the machine half of step 21: does the knowledge entry for a
// module still point at code that exists, and does the code carry rules the
// entry never recorded.
//
// Why this exists. The knowledge base's one failure mode that matters is
// staleness, not incompleteness. An entry that is missing is harmless — step
// 1 rebuilds AS-IS and says so. An entry that is wrong is taken as the
// starting AS-IS by every later run, and the run that trusts it never looks
// at the code the entry misdescribes. This checks the mechanical half: an
// enforcement anchor that no longer resolves, an identifier a registry
// declares and the entry records nowhere, a file the entry describes that
// has been deleted, a source file nothing describes, an entry past the
// 150-line budget, an INDEX.md row that disagrees with the directory, and
// commits touching the module since the entry was last updated. It decides
// nothing about meaning — whether a statement in the entry is true is step
// 21.4, done by hand against the code.
//
// Exit codes: 0 — every anchor resolves, every identifier is accounted for,
// the entry is within budget and the index agrees (or there is no entry yet,
// which is a first description, not a reconciliation); 1 — findings (step 21
// has material for its table); 2 — the command could not run (bad
// arguments, missing paths). Neither 1 nor 2 is a pass.

var kbRuleID = regexp.MustCompile(`\bR-\d+-\d+\b`)

var kbDefaultExt = []string{".go", ".py", ".ts", ".tsx", ".js", ".java", ".kt",
	".rb", ".rs", ".php", ".cs", ".sql", ".proto", ".yaml", ".yml"}

var kbSkipDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "__pycache__": true,
	".idea": true, ".venv": true, "dist": true, "build": true,
}

// kbPathish: a token in prose that looks like a path — at least one
// dot-suffix or slash, written in backticks.
var kbPathish = regexp.MustCompile("`([A-Za-z0-9_./-]+\\.[A-Za-z0-9_]{1,8}(?::[A-Za-z0-9_.]+)?)`")

var kbUpdatedRe = regexp.MustCompile(`(?m)^Updated:\s*(\S+)`)
var kbSliceRe = regexp.MustCompile(`Slice:\s*(\S+)`)
var kbTestNameRe = regexp.MustCompile(`_test\.|\.test\.|_spec\.|\.spec\.`)

func kbDie(env Env, format string, a ...any) error {
	fmt.Fprintf(env.Err, "kb-sync: "+format+"\n", a...)
	return exitCode(2)
}

func kbRead(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// kbTableRows returns the rows of the first markdown table under
// `## <heading>`, as lists of cells.
func kbTableRows(text, heading string) [][]string {
	var out [][]string
	inside, seenSep := false, false
	for _, line := range strings.Split(text, "\n") {
		if strings.HasPrefix(line, "## ") {
			if inside {
				break
			}
			inside = strings.HasPrefix(strings.ToLower(strings.TrimSpace(line[3:])), strings.ToLower(heading))
			continue
		}
		if !inside {
			continue
		}
		s := strings.TrimSpace(line)
		if !strings.HasPrefix(s, "|") {
			if len(out) > 0 {
				break
			}
			continue
		}
		cells := tcCellsOf(s)
		isSep := true
		for _, c := range cells {
			if strings.Trim(c, "-: ") != "" {
				isSep = false
				break
			}
		}
		if isSep {
			seenSep = true
			continue
		}
		if !seenSep {
			continue
		}
		out = append(out, cells)
	}
	return out
}

type kbEntryHow int

const (
	kbHowExplicit kbEntryHow = iota
	kbHowIndex
	kbHowFallback
	kbHowMissing
)

// kbResolveEntry finds the entry file through INDEX.md, matching on Project
// and Module both (a shared base holds several projects, and a module name
// is not unique across them), falling back to modules/<basename>.md only
// when the index has no row.
func kbResolveEntry(root, project, module, explicit string) (string, kbEntryHow) {
	if explicit != "" {
		return explicit, kbHowExplicit
	}
	index := filepath.Join(root, "INDEX.md")
	if data, err := kbRead(index); err == nil {
		for _, line := range strings.Split(data, "\n") {
			s := strings.TrimSpace(line)
			if !strings.HasPrefix(s, "|") {
				continue
			}
			var cells []string
			for _, c := range tcCellsOf(s) {
				cells = append(cells, strings.Trim(c, "`"))
			}
			if len(cells) >= 3 && cells[0] == project &&
				strings.TrimSuffix(cells[1], "/") == strings.TrimSuffix(module, "/") {
				return filepath.Join(root, cells[2]), kbHowIndex
			}
		}
	}
	guess := filepath.Join(root, project, "modules", filepath.Base(strings.TrimSuffix(module, "/"))+".md")
	if info, err := os.Stat(guess); err == nil && !info.IsDir() {
		return guess, kbHowFallback
	}
	return guess, kbHowMissing
}

func kbWalkSources(roots []string, exts map[string]bool) []string {
	var out []string
	for _, root := range roots {
		info, err := os.Stat(root)
		if err != nil {
			continue
		}
		if !info.IsDir() {
			out = append(out, root)
			continue
		}
		_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return nil
			}
			if d.IsDir() {
				name := d.Name()
				if path != root && (kbSkipDirs[name] || strings.HasPrefix(name, ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if exts[filepath.Ext(d.Name())] {
				out = append(out, path)
			}
			return nil
		})
	}
	return out
}

func kbWordIn(text, word string) bool {
	re := regexp.MustCompile(`\b` + regexp.QuoteMeta(word) + `\b`)
	return re.MatchString(text)
}

// kbGitSince reports commits touching module since the given date — a hint
// for where to look first, never a verdict (a commit may have changed
// nothing the entry describes). Returns nil, not an error, when git is
// unavailable or fails: the drift prior is a courtesy, not a gate.
func kbGitSince(repo, module, since string) []string {
	ctx, cancel := execTimeout(20 * time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", repo, "log",
		"--since="+since, "--format=%h %ad %s", "--date=short", "--", module)
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	lines := []string{}
	for _, l := range strings.Split(string(out), "\n") {
		if strings.TrimSpace(l) != "" {
			lines = append(lines, l)
		}
	}
	return lines
}

type kbFinding struct{ kind, msg string }

func runKBSync(env Env, args []string) error {
	fs := flag.NewFlagSet("kb-sync", flag.ContinueOnError)
	fs.SetOutput(env.Err)
	var module, knowledge, project, workdir, entry, repo, ext, search, format string
	var rulesArgs repeatedFlag
	fs.StringVar(&module, "module", "", "a directory path, the same string INDEX.md uses in its Module column")
	fs.Var(&rulesArgs, "rules", "repeatable; default: every registry under .pipeline/workdir/rules/")
	fs.StringVar(&knowledge, "knowledge", ".pipeline/knowledge", "root of the knowledge base; may hold several projects")
	fs.StringVar(&project, "project", "", "this project's directory inside the root; default: the repo directory's name")
	fs.StringVar(&workdir, "workdir", ".pipeline/workdir", "")
	fs.StringVar(&entry, "entry", "", "")
	fs.StringVar(&repo, "repo", ".", "")
	fs.StringVar(&ext, "ext", strings.Join(kbDefaultExt, ","), "")
	fs.StringVar(&search, "search", "", "extra directories to walk for identifiers and symbols")
	fs.StringVar(&format, "format", "text", "md or text")
	fs.Usage = func() {
		fmt.Fprint(env.Err, "Usage: trunkline kb-sync --module DIR [flags]\n\n"+
			"The machine half of step 21: does a knowledge entry still point at\n"+
			"code that exists. See core/developer/steps/21-reconciler.md.\n")
	}
	if err := fs.Parse(args); err != nil {
		return exitCode(2)
	}
	if module == "" {
		return kbDie(env, "--module is required")
	}

	module = strings.TrimSuffix(module, "/")
	moddir := filepath.Join(repo, module)
	if info, err := os.Stat(moddir); err != nil || !info.IsDir() {
		absRepo, _ := filepath.Abs(repo)
		return kbDie(env, "--module %s is not a directory under %s. "+
			"The scope of this route is a module path, not a name.", module, absRepo)
	}
	root := knowledge
	if !filepath.IsAbs(knowledge) {
		root = filepath.Join(repo, knowledge)
	}
	if project == "" {
		absRepo, _ := filepath.Abs(repo)
		project = filepath.Base(absRepo)
	}
	kb := filepath.Join(root, project)
	if info, err := os.Stat(root); err != nil || !info.IsDir() {
		return kbDie(env, "no knowledge base at %s. Either the `knowledge` flag names another "+
			"path, or the base has never been set up — step 20 bootstraps it, this command "+
			"does not.", kb)
	}

	entryPath, how := kbResolveEntry(root, project, module, entry)
	var findings []kbFinding
	var notes []string

	info, statErr := os.Stat(entryPath)
	if how == kbHowMissing || statErr != nil || info.IsDir() {
		rel, _ := filepath.Rel(repo, entryPath)
		fmt.Fprintf(env.Out, "NOTE     no entry for `%s` — neither INDEX.md nor "+
			"%s.\n"+
			"         This run is the module's FIRST description, not a reconciliation: there is\n"+
			"         nothing to contradict. Checks 1-4 are not applicable; step 21.4 writes the\n"+
			"         entry from the code and the supplied documents.\n", module, rel)
		fmt.Fprint(env.Out, "\nKB-SYNC: no entry — nothing to reconcile against (first description)\n")
		return nil
	}
	if how == kbHowFallback {
		rel, _ := filepath.Rel(repo, entryPath)
		notes = append(notes, fmt.Sprintf("INDEX.md has no row for `%s`; used %s "+
			"by name. The index disagrees with modules/ — a check 4 finding in itself.", module, rel))
		findings = append(findings, kbFinding{"INDEX", fmt.Sprintf("`%s` has an entry file but no row in INDEX.md", module)})
	}

	text, err := kbRead(entryPath)
	if err != nil {
		return kbDie(env, "cannot read %s: %v", entryPath, err)
	}
	nlines := strings.Count(text, "\n") + 1
	if strings.HasSuffix(text, "\n") {
		nlines--
	}
	if nlines > 150 {
		findings = append(findings, kbFinding{"SIZE", fmt.Sprintf("the entry is %d lines, past the 150-line budget — "+
			"rewrite it in full rather than patching it", nlines)})
	}

	exts := map[string]bool{}
	for _, e := range strings.Split(ext, ",") {
		if e == "" {
			continue
		}
		if !strings.HasPrefix(e, ".") {
			e = "." + e
		}
		exts[e] = true
	}
	roots := []string{moddir}
	for _, d := range strings.Split(search, ",") {
		if d != "" {
			roots = append(roots, filepath.Join(repo, d))
		}
	}
	var existingRoots []string
	for _, r := range roots {
		if _, err := os.Stat(r); err == nil {
			existingRoots = append(existingRoots, r)
		}
	}
	sourcesSet := map[string]bool{}
	for _, s := range kbWalkSources(existingRoots, exts) {
		sourcesSet[s] = true
	}
	var sources []string
	for s := range sourcesSet {
		sources = append(sources, s)
	}
	sort.Strings(sources)
	corpus := map[string]string{}
	for _, src := range sources {
		if t, err := kbRead(src); err == nil {
			corpus[src] = t
		}
	}

	// --- check 2: identifiers declared in the registries ---------------------
	rulesDir := filepath.Join(repo, workdir, "rules")
	registries := []string(rulesArgs)
	if len(registries) == 0 {
		if entries, err := os.ReadDir(rulesDir); err == nil {
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".md") {
					registries = append(registries, filepath.Join(rulesDir, e.Name()))
				}
			}
			sort.Strings(registries)
		}
	}
	declared := map[string]bool{}
	for _, reg := range registries {
		path := reg
		if !filepath.IsAbs(reg) {
			if _, err := os.Stat(reg); err != nil {
				path = filepath.Join(repo, reg)
			}
		}
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			notes = append(notes, fmt.Sprintf("registry %s not found — skipped", reg))
			continue
		}
		t, _ := kbRead(path)
		for _, id := range kbRuleID.FindAllString(t, -1) {
			declared[id] = true
		}
	}
	if len(registries) == 0 {
		notes = append(notes, "no rule registry found under .pipeline/workdir/rules/ — check 2 is not applicable: "+
			"this module's rules predate the registry, or its code never came from the "+
			"pipeline.")
	}

	rows := kbTableRows(text, "Business rules in force")
	var entryIDs []string
	type anchorRow struct{ rid, where string }
	var anchors []anchorRow
	for _, cells := range rows {
		if len(cells) == 0 {
			continue
		}
		rid := strings.Trim(cells[0], "` ")
		if !kbRuleID.MatchString(rid) || kbRuleID.FindString(rid) != rid {
			continue
		}
		entryIDs = append(entryIDs, rid)
		where := ""
		if len(cells) > 2 {
			where = cells[2]
		}
		anchors = append(anchors, anchorRow{rid, where})
	}

	for _, rid := range entryIDs {
		if len(declared) > 0 && !declared[rid] {
			findings = append(findings, kbFinding{"UNDECLARED", fmt.Sprintf("%s is in the entry's table but in no registry "+
				"under .pipeline/workdir/rules/", rid)})
		}
	}

	// --- check 1: anchors resolve --------------------------------------------
	locate := func(token string) string {
		cands := []string{filepath.Join(repo, token), filepath.Join(moddir, token)}
		for _, r := range existingRoots {
			cands = append(cands, filepath.Join(r, token))
		}
		for _, c := range cands {
			if info, err := os.Stat(c); err == nil && !info.IsDir() {
				return c
			}
		}
		base := filepath.Base(token)
		var hits []string
		for s := range corpus {
			if filepath.Base(s) == base {
				hits = append(hits, s)
			}
		}
		if len(hits) == 1 {
			return hits[0]
		}
		return ""
	}
	looksLikePath := func(token string) bool {
		path := token
		if i := strings.Index(token, ":"); i != -1 {
			path = token[:i]
		}
		return strings.Contains(path, "/") || exts[filepath.Ext(path)]
	}

	for _, a := range anchors {
		raw := kbPathish.FindAllStringSubmatch(a.where, -1)
		var tokens []string
		if len(raw) > 0 {
			for _, m := range raw {
				tokens = append(tokens, m[1])
			}
		} else {
			tokens = regexp.MustCompile(`[,\s]+`).Split(a.where, -1)
		}
		var filtered []string
		for _, t := range tokens {
			clean := strings.Trim(t, "`,. ")
			if clean != "" && looksLikePath(clean) {
				filtered = append(filtered, clean)
			}
		}
		if len(filtered) == 0 {
			shown := a.where
			if len(shown) > 48 {
				shown = shown[:48]
			}
			notes = append(notes, fmt.Sprintf("%s: `Where it is enforced` is prose (\"%s\") — nothing to "+
				"resolve. An anchor of the form `file:symbol` is what makes this "+
				"mechanical.", a.rid, shown))
			continue
		}
		for _, token := range filtered {
			path, sym, hasSym := strings.Cut(token, ":")
			found := locate(path)
			if found == "" {
				findings = append(findings, kbFinding{"DEAD", fmt.Sprintf("%s: anchor `%s` names `%s`, which does not "+
					"exist", a.rid, token, path)})
				continue
			}
			if !hasSym || sym == "" {
				continue
			}
			if isAllDigits(sym) {
				notes = append(notes, fmt.Sprintf("%s: anchor `%s` is a line number. Line anchors do not "+
					"survive a reformat — re-anchor to the symbol at that line.", a.rid, token))
				continue
			}
			body, ok := corpus[found]
			if !ok {
				body, _ = kbRead(found)
			}
			if !kbWordIn(body, sym) {
				rel, _ := filepath.Rel(repo, found)
				findings = append(findings, kbFinding{"STALE", fmt.Sprintf("%s: anchor `%s` — `%s` is no longer in "+
					"`%s`", a.rid, token, sym, rel)})
			}
		}
	}

	// --- check 3: the entry describes files that exist -----------------------
	mentionedSet := map[string]bool{}
	for _, m := range kbPathish.FindAllStringSubmatch(text, -1) {
		token, _, _ := strings.Cut(m[1], ":")
		mentionedSet[token] = true
	}
	var mentioned []string
	for m := range mentionedSet {
		mentioned = append(mentioned, m)
	}
	sort.Strings(mentioned)
	for _, token := range mentioned {
		if !exts[filepath.Ext(token)] && !strings.Contains(token, "/") {
			continue
		}
		if locate(token) == "" {
			findings = append(findings, kbFinding{"GONE", fmt.Sprintf("the entry names `%s`, which does not exist", token)})
		}
	}
	described := map[string]bool{}
	for m := range mentionedSet {
		described[m] = true
		described[filepath.Base(m)] = true
	}
	var undescribed []string
	modPrefix := moddir + string(filepath.Separator)
	for _, src := range sources {
		if !strings.HasPrefix(src, modPrefix) && src != moddir {
			continue
		}
		rel, _ := filepath.Rel(repo, src)
		if described[filepath.Base(src)] || described[rel] {
			continue
		}
		if kbTestNameRe.MatchString(filepath.Base(src)) {
			continue
		}
		undescribed = append(undescribed, rel)
	}
	if len(undescribed) > 0 {
		shown := undescribed
		tail := ""
		if len(shown) > 6 {
			shown = shown[:6]
			tail = ", …"
		}
		findings = append(findings, kbFinding{"UNDESCRIBED", fmt.Sprintf("%d source file(s) of the module are named "+
			"nowhere in the entry: %s%s", len(undescribed), strings.Join(shown, ", "), tail)})
	}

	// --- check 4: INDEX.md agrees --------------------------------------------
	indexPath := filepath.Join(root, "INDEX.md")
	updated := ""
	if m := kbUpdatedRe.FindStringSubmatch(text); m != nil {
		updated = m[1]
	} else {
		findings = append(findings, kbFinding{"HEADER", "the entry has no `Updated:` line — the drift report has no " +
			"date to start from"})
	}
	if m := kbSliceRe.FindStringSubmatch(text); m != nil {
		slug := strings.Trim(m[1], "`")
		slicesDir := filepath.Join(kb, "slices")
		if entries, err := os.ReadDir(slicesDir); err == nil {
			found := false
			for _, e := range entries {
				if strings.HasPrefix(e.Name(), slug) || strings.Contains(e.Name(), slug) {
					found = true
					break
				}
			}
			if !found {
				findings = append(findings, kbFinding{"INDEX", fmt.Sprintf("the entry names slice `%s`, which is not in "+
					"%s/%s/slices/", slug, knowledge, project)})
			}
		}
	}
	if idx, err := kbRead(indexPath); err == nil {
		want, _ := filepath.Rel(root, entryPath)
		want = strings.ReplaceAll(want, string(filepath.Separator), "/")
		if how != kbHowFallback && !strings.Contains(idx, want) && !strings.Contains(idx, filepath.Base(entryPath)) {
			findings = append(findings, kbFinding{"INDEX", fmt.Sprintf("INDEX.md has no row for %s", want)})
		}
	} else {
		findings = append(findings, kbFinding{"INDEX", fmt.Sprintf("no INDEX.md at the root of the base (%s)", knowledge)})
	}

	// --- the drift prior ------------------------------------------------------
	if updated != "" {
		commits := kbGitSince(repo, module, updated)
		if commits == nil {
			notes = append(notes, "git log unavailable — the drift prior is skipped; it is a hint, not a gate.")
		} else if len(commits) > 0 {
			notes = append(notes, fmt.Sprintf("%d commit(s) touched `%s` since the entry was updated "+
				"(%s). Not a verdict — read them first:", len(commits), module, updated))
			shown := commits
			tail := false
			if len(shown) > 8 {
				shown = shown[:8]
				tail = true
			}
			for _, line := range shown {
				notes = append(notes, "    "+line)
			}
			if tail {
				notes = append(notes, fmt.Sprintf("    … %d more", len(commits)-8))
			}
		}
	} else {
		notes = append(notes, "git log unavailable — the drift prior is skipped; it is a hint, not a gate.")
	}

	// --- output ---------------------------------------------------------------
	relEntry, _ := filepath.Rel(repo, entryPath)
	header := fmt.Sprintf("entry %s (%d lines) — module `%s`, %d source file(s) searched, %d rule(s) in the table",
		relEntry, nlines, module, len(sources), len(entryIDs))
	if format == "md" {
		fmt.Fprintf(env.Out, "**kb-sync** — %s\n\n", header)
		fmt.Fprintln(env.Out, "| Kind | Finding |")
		fmt.Fprintln(env.Out, "|---|---|")
		for _, f := range findings {
			fmt.Fprintf(env.Out, "| %s | %s |\n", f.kind, f.msg)
		}
	} else {
		fmt.Fprintf(env.Out, "%s\n\n", header)
		for _, f := range findings {
			fmt.Fprintf(env.Out, "%-12s %s\n", f.kind, f.msg)
		}
	}
	if len(notes) > 0 {
		fmt.Fprintln(env.Out)
		for _, n := range notes {
			if strings.HasPrefix(n, "    ") {
				fmt.Fprintln(env.Out, n)
			} else {
				fmt.Fprintf(env.Out, "NOTE     %s\n", n)
			}
		}
	}

	verdict := "ANCHORS CLEAN"
	if len(findings) > 0 {
		verdict = "DRIFT"
	}
	fmt.Fprintf(env.Out, "\nKB-SYNC: %d finding(s) — %s\n", len(findings), verdict)
	fmt.Fprint(env.Out, "Mechanical only: the entry points at code that exists and no identifier is unaccounted "+
		"for. Whether what the entry SAYS is true — contradictions, missing rules, stale prose — "+
		"is step 21.4, and it is still yours.\n")

	if len(findings) > 0 {
		return exitCode(1)
	}
	return nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
