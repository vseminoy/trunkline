package cli

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	trunkline "github.com/vseminoy/trunkline"
)

// token-cost — what one run actually cost, in tokens by kind and in
// dollars.
//
// Why this exists. The journal answers "what did this slice cost" in
// confirmations, rollbacks and minutes. Those are process costs. The other
// cost — the one that decides whether a 21-step route is worth running on a
// two-line fix — is not in the journal and cannot be, because the agent
// does not see its own token usage. It is in the host's transcript, and
// every host writes a different one. This reads Claude Code's.
//
// The dedup is the whole correctness story: one API response is written to
// the transcript as several records, one per content block, and every one
// of them carries the same message.usage. Summing the records instead of
// the calls overstates the total by 2-9x on a normal agentic run, and it
// overstates it plausibly — nothing about the number looks wrong. Records
// are collapsed by message.id before anything is added up.
//
// Exit codes: 0 — a report was produced; 2 — no report could be produced
// (no transcript directory, no calls in the selected window, no such slice
// in the journal); 1 is not used, on purpose — the report is an input to a
// decision about the route, not a gate on a step.

const tcStaleAfterDays = 92

type tcKind struct {
	key, label string
	mult       *float64 // nil = output rate
}

type tcPrices struct {
	asOf         string
	models       map[string]struct{ in, out float64 }
	cacheWrite5m float64
	cacheWrite1h float64
	cacheRead    float64
}

func tcKinds(p tcPrices) []tcKind {
	w5, w1, cr := p.cacheWrite5m, p.cacheWrite1h, p.cacheRead
	return []tcKind{
		{"input", "input", floatPtr(1.0)},
		{"cache_write_5m", "cache write 5m", &w5},
		{"cache_write_1h", "cache write 1h", &w1},
		{"cache_read", "cache read", &cr},
		{"output", "output", nil},
	}
}

func floatPtr(f float64) *float64 { return &f }

type priceFile struct {
	AsOf   string `json:"as_of"`
	Models map[string]struct {
		In  float64 `json:"in"`
		Out float64 `json:"out"`
	} `json:"models"`
	Multipliers struct {
		CacheWrite5m *float64 `json:"cache_write_5m"`
		CacheWrite1h *float64 `json:"cache_write_1h"`
		CacheRead    *float64 `json:"cache_read"`
	} `json:"multipliers"`
}

func tcLoadPrices(data []byte) (tcPrices, error) {
	var pf priceFile
	if err := json.Unmarshal(data, &pf); err != nil {
		return tcPrices{}, err
	}
	if len(pf.Models) == 0 {
		return tcPrices{}, fmt.Errorf("models")
	}
	p := tcPrices{
		asOf:         pf.AsOf,
		models:       map[string]struct{ in, out float64 }{},
		cacheWrite5m: 1.25,
		cacheWrite1h: 2.00,
		cacheRead:    0.10,
	}
	if p.asOf == "" {
		p.asOf = "unknown"
	}
	for k, v := range pf.Models {
		p.models[k] = struct{ in, out float64 }{v.In, v.Out}
	}
	if pf.Multipliers.CacheWrite5m != nil {
		p.cacheWrite5m = *pf.Multipliers.CacheWrite5m
	}
	if pf.Multipliers.CacheWrite1h != nil {
		p.cacheWrite1h = *pf.Multipliers.CacheWrite1h
	}
	if pf.Multipliers.CacheRead != nil {
		p.cacheRead = *pf.Multipliers.CacheRead
	}
	return p, nil
}

func tcPricesAgeDays(asOf string) (int, bool) {
	taken, err := time.Parse("2006-01-02", asOf)
	if err != nil {
		return 0, false
	}
	return int(time.Since(taken).Hours() / 24), true
}

func tcStalenessWarning(p tcPrices, defaultName string) string {
	age, ok := tcPricesAgeDays(p.asOf)
	if !ok || age <= tcStaleAfterDays {
		return ""
	}
	months := int(math.Round(float64(age) / 30.44))
	return fmt.Sprintf("!! These prices are %d months old (taken %s). A model whose "+
		"rate changed since is priced here at the old one, silently. Check "+
		"anthropic.com/pricing and update %s, or pass --prices.", months, p.asOf, defaultName)
}

// tcRateFor is a longest-prefix match into the price table.
func tcRateFor(p tcPrices, model string) (struct{ in, out float64 }, bool) {
	var best string
	var bestRates struct{ in, out float64 }
	found := false
	for prefix, rates := range p.models {
		if strings.HasPrefix(model, prefix) && (!found || len(prefix) > len(best)) {
			best, bestRates, found = prefix, rates, true
		}
	}
	return bestRates, found
}

func tcKindRate(rates struct{ in, out float64 }, mult *float64) float64 {
	if mult == nil {
		return rates.out
	}
	return rates.in * *mult
}

var tcSlugNonAlnum = regexp.MustCompile(`[^a-zA-Z0-9]`)

func tcSlugifyProject(abs string) string {
	return tcSlugNonAlnum.ReplaceAllString(abs, "-")
}

// tcFindTranscriptDir locates the transcript directory, returning (dir, how
// it was found) — dir is "" when none exists.
func tcFindTranscriptDir(project, override string) (string, string) {
	configDir := os.Getenv("CLAUDE_CONFIG_DIR")
	home, _ := os.UserHomeDir()
	root := filepath.Join(home, ".claude", "projects")
	if configDir != "" {
		root = filepath.Join(configDir, "projects")
	}
	if override != "" {
		p := expandUser(override)
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			return p, fmt.Sprintf("--transcripts %s", override)
		}
		return "", fmt.Sprintf("--transcripts %s", override)
	}
	absProject, _ := filepath.Abs(project)
	direct := filepath.Join(root, tcSlugifyProject(absProject))
	if info, err := os.Stat(direct); err == nil && info.IsDir() {
		return direct, direct
	}
	if entries, err := os.ReadDir(root); err == nil {
		var dirs []string
		for _, e := range entries {
			if e.IsDir() {
				dirs = append(dirs, e.Name())
			}
		}
		sort.Strings(dirs)
		for _, d := range dirs {
			cand := filepath.Join(root, d)
			matches, _ := filepath.Glob(filepath.Join(cand, "*.jsonl"))
			for _, f := range matches {
				if tcFirstCwd(f) == absProject {
					return cand, fmt.Sprintf("%s (matched by cwd; project path has moved)", cand)
				}
			}
		}
	}
	return "", fmt.Sprintf("%s (does not exist)", direct)
}

func expandUser(p string) string {
	if strings.HasPrefix(p, "~") {
		home, _ := os.UserHomeDir()
		return filepath.Join(home, strings.TrimPrefix(p, "~"))
	}
	return p
}

func tcFirstCwd(path string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 10*1024*1024)
	for sc.Scan() {
		var rec map[string]any
		if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
			continue
		}
		if cwd, ok := rec["cwd"].(string); ok && cwd != "" {
			return cwd
		}
	}
	return ""
}

func tcParseTS(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	v := strings.TrimSpace(value)
	v = strings.ReplaceAll(v, "Z", "+00:00")
	for _, layout := range []string{time.RFC3339, "2006-01-02T15:04:05.999999999-07:00", "2006-01-02T15:04:05-07:00"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}

type tcCall struct {
	session                                                        string
	model                                                          string
	ts                                                             time.Time
	hasTS                                                          bool
	sidechain                                                      bool
	input, cacheWrite5m, cacheWrite1h, cacheRead, output, thinking int
}

func tcCallTotal(c tcCall) int {
	return c.input + c.cacheWrite5m + c.cacheWrite1h + c.cacheRead + c.output
}

// tcReadCalls yields one call per message.id, deduplicated — several
// transcript records share an id (one per content block) and repeat the
// same usage; where repeats disagree, the largest is kept.
func tcReadCalls(directory string, sessions map[string]bool) ([]tcCall, error) {
	matches, err := filepath.Glob(filepath.Join(directory, "*.jsonl"))
	if err != nil {
		return nil, err
	}
	sort.Strings(matches)
	calls := map[string]tcCall{}
	order := []string{}
	for _, path := range matches {
		session := strings.TrimSuffix(filepath.Base(path), ".jsonl")
		if sessions != nil && !sessions[session] {
			continue
		}
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		sc := bufio.NewScanner(f)
		sc.Buffer(make([]byte, 0, 64*1024), 50*1024*1024)
		for sc.Scan() {
			var rec map[string]any
			if err := json.Unmarshal(sc.Bytes(), &rec); err != nil {
				continue
			}
			if rec["type"] != "assistant" {
				continue
			}
			msg, ok := rec["message"].(map[string]any)
			if !ok {
				continue
			}
			usage, ok := msg["usage"].(map[string]any)
			if !ok {
				continue
			}
			model, _ := msg["model"].(string)
			if model == "" {
				model = "unknown"
			}
			if model == "<synthetic>" {
				continue
			}
			key, _ := msg["id"].(string)
			if key == "" {
				key, _ = rec["requestId"].(string)
			}
			if key == "" {
				key, _ = rec["uuid"].(string)
			}
			if key == "" {
				continue
			}
			var w5, w1 float64
			haveSplit := false
			if cc, ok := usage["cache_creation"].(map[string]any); ok {
				if v, ok := cc["ephemeral_5m_input_tokens"]; ok {
					w5 = toFloat(v)
					haveSplit = true
				}
				if v, ok := cc["ephemeral_1h_input_tokens"]; ok {
					w1 = toFloat(v)
					haveSplit = true
				}
			}
			if !haveSplit {
				w5 = toFloat(usage["cache_creation_input_tokens"])
				w1 = 0
			}
			var details map[string]any
			if d, ok := usage["output_tokens_details"].(map[string]any); ok {
				details = d
			}
			ts, hasTS := tcParseTS(stringOf(rec["timestamp"]))
			call := tcCall{
				session:      session,
				model:        model,
				ts:           ts,
				hasTS:        hasTS,
				sidechain:    boolOf(rec["isSidechain"]),
				input:        int(toFloat(usage["input_tokens"])),
				cacheWrite5m: int(w5),
				cacheWrite1h: int(w1),
				cacheRead:    int(toFloat(usage["cache_read_input_tokens"])),
				output:       int(toFloat(usage["output_tokens"])),
				thinking:     int(toFloat(details["thinking_tokens"])),
			}
			if prev, ok := calls[key]; !ok || tcCallTotal(call) > tcCallTotal(prev) {
				if !ok {
					order = append(order, key)
				}
				calls[key] = call
			}
		}
		f.Close()
	}
	out := make([]tcCall, 0, len(order))
	for _, k := range order {
		out = append(out, calls[k])
	}
	return out, nil
}

func toFloat(v any) float64 {
	switch n := v.(type) {
	case float64:
		return n
	case int:
		return float64(n)
	default:
		return 0
	}
}
func stringOf(v any) string {
	s, _ := v.(string)
	return s
}
func boolOf(v any) bool {
	b, _ := v.(bool)
	return b
}

// tcSliceWindow reads a slice's window out of the journal: (since, until,
// sessions, note).
func tcSliceWindow(journal, sliceID string) (time.Time, bool, time.Time, bool, map[string]bool, string, error) {
	if info, err := os.Stat(journal); err != nil || info.IsDir() {
		return time.Time{}, false, time.Time{}, false, nil, "", fmt.Errorf("journal not found: %s", journal)
	}
	b, err := os.ReadFile(journal)
	if err != nil {
		return time.Time{}, false, time.Time{}, false, nil, "", err
	}
	var start, done time.Time
	var hasStart, hasDone bool
	sessions := map[string]bool{}
	seen := false
	for _, line := range strings.Split(string(b), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(line), &ev); err != nil {
			continue
		}
		sl := fmt.Sprintf("%v", ev["slice"])
		if sl != sliceID {
			continue
		}
		seen = true
		if sid, ok := ev["session_id"]; ok && sid != nil && fmt.Sprintf("%v", sid) != "" {
			sessions[fmt.Sprintf("%v", sid)] = true
		}
		if ev["event"] == "start" {
			if t, ok := tcParseTS(stringOf(ev["ts"])); ok {
				start, hasStart = t, true
			}
		} else if ev["event"] == "done" {
			if t, ok := tcParseTS(stringOf(ev["ts"])); ok {
				done, hasDone = t, true
			}
		}
	}
	if !seen {
		return time.Time{}, false, time.Time{}, false, nil, "", fmt.Errorf("slice %s has no events in %s", sliceID, journal)
	}
	if !hasStart {
		return time.Time{}, false, time.Time{}, false, nil, "", fmt.Errorf("slice %s has no `start` event with a usable ts", sliceID)
	}
	var note string
	switch {
	case len(sessions) > 0:
		note = "bound by session_id from the journal — exact"
	case !hasDone:
		note = "no `done` event: the run is unfinished, or step 20 did not write one"
	default:
		note = "bound by the journal's wall-clock window — anything else done in this " +
			"project between those timestamps is counted in"
	}
	return start, hasStart, done, hasDone, sessions, note, nil
}

type tcModelAgg struct {
	calls, thinking                                      int
	input, cacheWrite5m, cacheWrite1h, cacheRead, output int
}

func tcAggregate(calls []tcCall) map[string]*tcModelAgg {
	byModel := map[string]*tcModelAgg{}
	for _, c := range calls {
		acc, ok := byModel[c.model]
		if !ok {
			acc = &tcModelAgg{}
			byModel[c.model] = acc
		}
		acc.input += c.input
		acc.cacheWrite5m += c.cacheWrite5m
		acc.cacheWrite1h += c.cacheWrite1h
		acc.cacheRead += c.cacheRead
		acc.output += c.output
		acc.calls++
		acc.thinking += c.thinking
	}
	return byModel
}

func tcAggTokens(a *tcModelAgg, key string) int {
	switch key {
	case "input":
		return a.input
	case "cache_write_5m":
		return a.cacheWrite5m
	case "cache_write_1h":
		return a.cacheWrite1h
	case "cache_read":
		return a.cacheRead
	case "output":
		return a.output
	}
	return 0
}

type tcPricedKind struct {
	label  string
	tokens int
	rate   *float64
	cost   *float64
}

type tcPricedModel struct {
	calls, thinking, tokens int
	kinds                   []tcPricedKind
	subtotal                *float64
	priced                  bool
}

func tcPrice(p tcPrices, byModel map[string]*tcModelAgg) map[string]tcPricedModel {
	priced := map[string]tcPricedModel{}
	kinds := tcKinds(p)
	for model, acc := range byModel {
		rates, ok := tcRateFor(p, model)
		var subtotal *float64
		if ok {
			z := 0.0
			subtotal = &z
		}
		var pk []tcPricedKind
		tokensTotal := 0
		for _, k := range kinds {
			toks := tcAggTokens(acc, k.key)
			tokensTotal += toks
			var rate, cost *float64
			if ok {
				r := tcKindRate(rates, k.mult)
				rate = &r
				c := float64(toks) / 1_000_000 * r
				cost = &c
				*subtotal += c
			}
			pk = append(pk, tcPricedKind{k.label, toks, rate, cost})
		}
		priced[model] = tcPricedModel{
			calls: acc.calls, thinking: acc.thinking, tokens: tokensTotal,
			kinds: pk, subtotal: subtotal, priced: ok,
		}
	}
	return priced
}

func tcN(v int) string {
	s := fmt.Sprintf("%d", v)
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	parts = append([]string{s}, parts...)
	out := strings.Join(parts, " ")
	if neg {
		out = "-" + out
	}
	return out
}

func tcMoney(v *float64) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("$%.2f", *v)
}

func tcPlural(n int, word string) string {
	if n == 1 {
		return word
	}
	return word + "s"
}

func tcRenderText(header []string, priced map[string]tcPricedModel, totalTokens int, totalCost *float64,
	sidechainTokens int, unpriced []string, p tcPrices, defaultName string) string {
	var out []string
	out = append(out, header...)
	out = append(out, "")
	var models []string
	for m := range priced {
		models = append(models, m)
	}
	sort.Strings(models)
	for _, model := range models {
		block := priced[model]
		out = append(out, fmt.Sprintf("  %s  (%s %s)", model, tcN(block.calls), tcPlural(block.calls, "call")))
		for _, k := range block.kinds {
			rate := "—"
			if k.rate != nil {
				rate = fmt.Sprintf("%.2f", *k.rate)
			}
			out = append(out, fmt.Sprintf("    %-15s%16s%12s $/MTok%12s", k.label, tcN(k.tokens), rate, tcMoney(k.cost)))
		}
		out = append(out, fmt.Sprintf("    %-15s%16s%19s%12s", "subtotal", tcN(block.tokens), "", tcMoney(block.subtotal)))
		if block.thinking > 0 {
			out = append(out, fmt.Sprintf("    (output includes %s thinking tokens)", tcN(block.thinking)))
		}
		if !block.priced {
			out = append(out, fmt.Sprintf("    !! no price for %s — tokens counted, cost not", model))
		}
		out = append(out, "")
	}
	out = append(out, fmt.Sprintf("  TOTAL%26s tokens%13s%12s", tcN(totalTokens), "", tcMoney(totalCost)))
	if sidechainTokens > 0 {
		share := 0.0
		if totalTokens > 0 {
			share = float64(sidechainTokens) / float64(totalTokens) * 100
		}
		out = append(out, fmt.Sprintf("  of which subagents%13s tokens (%.0f%%)", tcN(sidechainTokens), share))
	}
	out = append(out, "")
	out = append(out, fmt.Sprintf("  Prices as of %s, Anthropic list, USD. Cache write "+
		"%sx (5m) / %sx (1h) input, read %sx.", p.asOf, trimFloat(p.cacheWrite5m), trimFloat(p.cacheWrite1h), trimFloat(p.cacheRead)))
	if len(unpriced) > 0 {
		sort.Strings(unpriced)
		out = append(out, fmt.Sprintf("  No price for %s — those tokens are "+
			"counted, their cost is not in the total. Add them with --prices.", strings.Join(unpriced, ", ")))
	}
	if stale := tcStalenessWarning(p, defaultName); stale != "" {
		out = append(out, "", "  "+stale)
	}
	return strings.Join(out, "\n")
}

func trimFloat(f float64) string {
	s := strconv2(f)
	return s
}

func strconv2(f float64) string {
	s := fmt.Sprintf("%g", f)
	return s
}

func tcRenderMD(header []string, priced map[string]tcPricedModel, totalTokens int, totalCost *float64,
	unpriced []string, p tcPrices, defaultName string) string {
	var out []string
	out = append(out, fmt.Sprintf("**%s**", header[0]), "")
	for _, line := range header[1:] {
		if strings.TrimSpace(line) != "" {
			out = append(out, "- "+strings.TrimSpace(line))
		}
	}
	out = append(out, "", "| Model | Kind | Tokens | $/MTok | Cost |", "|---|---|---:|---:|---:|")
	var models []string
	for m := range priced {
		models = append(models, m)
	}
	sort.Strings(models)
	for _, model := range models {
		block := priced[model]
		for i, k := range block.kinds {
			rate := "—"
			if k.rate != nil {
				rate = fmt.Sprintf("%.2f", *k.rate)
			}
			name := ""
			if i == 0 {
				name = fmt.Sprintf("`%s`", model)
			}
			out = append(out, fmt.Sprintf("| %s | %s | %s | %s | %s |", name, k.label, tcN(k.tokens), rate, tcMoney(k.cost)))
		}
		out = append(out, fmt.Sprintf("| | **subtotal** | **%s** | | **%s** |", tcN(block.tokens), tcMoney(block.subtotal)))
	}
	out = append(out, fmt.Sprintf("| | **TOTAL** | **%s** | | **%s** |", tcN(totalTokens), tcMoney(totalCost)))
	out = append(out, "")
	out = append(out, fmt.Sprintf("Prices as of %s, Anthropic list, USD. "+
		"Cache write %sx (5m) / %sx (1h) input, read %sx.", p.asOf, trimFloat(p.cacheWrite5m), trimFloat(p.cacheWrite1h), trimFloat(p.cacheRead)))
	if len(unpriced) > 0 {
		sort.Strings(unpriced)
		out = append(out, fmt.Sprintf("No price for %s — those tokens are "+
			"counted, their cost is not in the total. Add them with `--prices`.", strings.Join(unpriced, ", ")))
	}
	if stale := tcStalenessWarning(p, defaultName); stale != "" {
		out = append(out, "", fmt.Sprintf("**%s**", stale))
	}
	return strings.Join(out, "\n")
}

func runTokenCost(env Env, args []string) error {
	fs := flag.NewFlagSet("token-cost", flag.ContinueOnError)
	fs.SetOutput(env.Err)
	var slice, session, since, until string
	var allFlag bool
	var project, journal, transcripts, pricesPath, format string
	fs.StringVar(&slice, "slice", "", "one run, bound through the journal")
	fs.StringVar(&session, "session", "", "one session; `current` reads CLAUDE_CODE_SESSION_ID")
	fs.StringVar(&since, "since", "", "ISO-8601 lower bound (with optional --until)")
	fs.BoolVar(&allFlag, "all", false, "every session recorded for this project")
	fs.StringVar(&until, "until", "", "ISO-8601 upper bound; only with --since")
	fs.StringVar(&project, "project", ".", "project root (default: cwd)")
	fs.StringVar(&journal, "journal", ".pipeline/workdir/metrics/pipeline.jsonl", "")
	fs.StringVar(&transcripts, "transcripts", "", "transcript directory, if not the default location")
	fs.StringVar(&pricesPath, "prices", "", "price table to use instead of the shipped one")
	fs.StringVar(&format, "format", "text", "text, md or json")
	fs.Usage = func() {
		fmt.Fprint(env.Err, "Usage: trunkline token-cost (--slice NNN|--session ID|--since TS|--all) [flags]\n\n"+
			"What a run cost, by token kind and in dollars, from Claude Code's own\n"+
			"transcripts. See core/developer/references/pipeline-evals.md.\n")
	}
	if err := fs.Parse(args); err != nil {
		return exitCode(2)
	}
	selectors := 0
	for _, s := range []bool{slice != "", session != "", since != "", allFlag} {
		if s {
			selectors++
		}
	}
	if selectors != 1 {
		fmt.Fprintln(env.Err, "token-cost: exactly one of --slice, --session, --since or --all is required")
		return exitCode(2)
	}

	const defaultPricesName = "prices.json"
	var raw []byte
	var err error
	if pricesPath != "" {
		raw, err = os.ReadFile(pricesPath)
	} else {
		raw, err = trunkline.Core.ReadFile("core/developer/scripts/prices.json")
	}
	var prices tcPrices
	if err == nil {
		prices, err = tcLoadPrices(raw)
	}
	if err != nil {
		which := fmt.Sprintf("the price table %s", defaultPricesName)
		if pricesPath != "" {
			which = fmt.Sprintf("--prices %s", pricesPath)
		}
		fmt.Fprintf(env.Err, "token-cost: cannot read %s: %v\n", which, err)
		return exitCode(2)
	}

	directory, how := tcFindTranscriptDir(project, transcripts)
	if directory == "" {
		fmt.Fprintf(env.Err, "token-cost: no Claude Code transcripts for this project.\n"+
			"  looked in: %s\n"+
			"  This host does not record usage where this command can read it; the run\n"+
			"  happened, its cost is simply not measurable here.\n", how)
		return exitCode(2)
	}

	var sinceT, untilT time.Time
	var hasSince, hasUntil bool
	var sessions map[string]bool
	note, scope := "", ""

	switch {
	case slice != "":
		s, hs, u, hu, journalSessions, n, err := tcSliceWindow(journal, slice)
		if err != nil {
			fmt.Fprintf(env.Err, "token-cost: %v\n", err)
			return exitCode(2)
		}
		sinceT, hasSince, untilT, hasUntil, note = s, hs, u, hu, n
		if len(journalSessions) > 0 {
			sessions = journalSessions
		}
		scope = fmt.Sprintf("SLICE %s", slice)
	case session != "":
		sid := session
		if sid == "current" {
			sid = os.Getenv("CLAUDE_CODE_SESSION_ID")
			if sid == "" {
				fmt.Fprintln(env.Err, "token-cost: --session current needs CLAUDE_CODE_SESSION_ID, which is set\n"+
					"  inside a Claude Code shell. Pass the id explicitly instead.")
				return exitCode(2)
			}
		}
		sessions = map[string]bool{sid: true}
		scope = fmt.Sprintf("SESSION %s", sid)
		note = "the reply that runs this command is not in the transcript yet"
	case since != "":
		s, ok := tcParseTS(since)
		if !ok {
			fmt.Fprintln(env.Err, "token-cost: --since/--until must be ISO-8601")
			return exitCode(2)
		}
		sinceT, hasSince = s, true
		if until != "" {
			u, ok := tcParseTS(until)
			if !ok {
				fmt.Fprintln(env.Err, "token-cost: --since/--until must be ISO-8601")
				return exitCode(2)
			}
			untilT, hasUntil = u, true
		}
		scope = "WINDOW"
		note = "everything this project did in the window, whatever it was for"
	default:
		scope = "ALL RUNS"
	}

	allCalls, err := tcReadCalls(directory, sessions)
	if err != nil {
		fmt.Fprintf(env.Err, "token-cost: %v\n", err)
		return exitCode(2)
	}
	var calls []tcCall
	for _, c := range allCalls {
		if hasSince && (!c.hasTS || c.ts.Before(sinceT)) {
			continue
		}
		if hasUntil && (!c.hasTS || c.ts.After(untilT)) {
			continue
		}
		calls = append(calls, c)
	}
	if len(calls) == 0 {
		fmt.Fprintf(env.Err, "token-cost: no API calls in scope (%s). Nothing to report.\n", scope)
		return exitCode(2)
	}

	byModel := tcAggregate(calls)
	priced := tcPrice(prices, byModel)
	totalTokens := 0
	var subtotals []float64
	for _, b := range priced {
		totalTokens += b.tokens
		if b.subtotal != nil {
			subtotals = append(subtotals, *b.subtotal)
		}
	}
	var totalCost *float64
	if len(subtotals) > 0 {
		s := 0.0
		for _, v := range subtotals {
			s += v
		}
		totalCost = &s
	}
	sidechainTokens := 0
	for _, c := range calls {
		if c.sidechain {
			sidechainTokens += tcCallTotal(c)
		}
	}
	var unpriced []string
	for m, b := range priced {
		if !b.priced {
			unpriced = append(unpriced, m)
		}
	}

	var span string
	var stamps []time.Time
	for _, c := range calls {
		if c.hasTS {
			stamps = append(stamps, c.ts)
		}
	}
	if len(stamps) > 0 {
		lo, hi := stamps[0], stamps[0]
		for _, t := range stamps {
			if t.Before(lo) {
				lo = t
			}
			if t.After(hi) {
				hi = t
			}
		}
		minutes := int(math.Round(hi.Sub(lo).Minutes()))
		span = fmt.Sprintf("%s → %s (%d min)", lo.Format(time.RFC3339), hi.Format(time.RFC3339), minutes)
	}

	usedSet := map[string]bool{}
	for _, c := range calls {
		usedSet[c.session] = true
	}
	var used []string
	for s := range usedSet {
		used = append(used, s)
	}
	sort.Strings(used)
	var usedShown []string
	for _, s := range used {
		if len(s) > 8 {
			usedShown = append(usedShown, s[:8])
		} else {
			usedShown = append(usedShown, s)
		}
	}

	headerLine := fmt.Sprintf("%s — %d API %s across %d %s",
		scope, len(calls), tcPlural(len(calls), "call"), len(used), tcPlural(len(used), "session"))
	var header []string
	header = append(header, headerLine)
	if span != "" {
		header = append(header, "  "+span)
	}
	header = append(header, "  sessions: "+strings.Join(usedShown, ", "))
	if note != "" {
		header = append(header, "  "+note)
	}

	switch format {
	case "json":
		type jsonKind struct {
			Label  string   `json:"label"`
			Tokens int      `json:"tokens"`
			Rate   *float64 `json:"rate"`
			Cost   *float64 `json:"cost"`
		}
		type jsonModel struct {
			Calls    int                 `json:"calls"`
			Thinking int                 `json:"thinking"`
			Tokens   int                 `json:"tokens"`
			Kinds    map[string]jsonKind `json:"kinds"`
			Subtotal *float64            `json:"subtotal"`
			Priced   bool                `json:"priced"`
		}
		byModelJSON := map[string]jsonModel{}
		kindKeys := []string{"input", "cache_write_5m", "cache_write_1h", "cache_read", "output"}
		for model, b := range priced {
			kinds := map[string]jsonKind{}
			for i, k := range b.kinds {
				kinds[kindKeys[i]] = jsonKind{k.label, k.tokens, k.rate, k.cost}
			}
			byModelJSON[model] = jsonModel{b.calls, b.thinking, b.tokens, kinds, b.subtotal, b.priced}
		}
		age, ok := tcPricesAgeDays(prices.asOf)
		var ageVal any
		if ok {
			ageVal = age
		}
		var sinceStr, untilStr any
		if hasSince {
			sinceStr = sinceT.Format(time.RFC3339)
		}
		if hasUntil {
			untilStr = untilT.Format(time.RFC3339)
		}
		sort.Strings(unpriced)
		out := map[string]any{
			"scope":           scope,
			"note":            note,
			"sessions":        used,
			"since":           sinceStr,
			"until":           untilStr,
			"prices_as_of":    prices.asOf,
			"prices_age_days": ageVal,
			"prices_stale":    tcStalenessWarning(prices, defaultPricesName) != "",
			"multipliers": map[string]float64{
				"cache_write_5m": prices.cacheWrite5m,
				"cache_write_1h": prices.cacheWrite1h,
				"cache_read":     prices.cacheRead,
			},
			"by_model": byModelJSON,
			"totals": map[string]any{
				"tokens":           totalTokens,
				"cost":             totalCost,
				"sidechain_tokens": sidechainTokens,
				"calls":            len(calls),
				"unpriced_models":  unpriced,
			},
		}
		enc := json.NewEncoder(env.Out)
		enc.SetIndent("", "  ")
		enc.SetEscapeHTML(false)
		_ = enc.Encode(out)
	case "md":
		fmt.Fprintln(env.Out, tcRenderMD(header, priced, totalTokens, totalCost, unpriced, prices, defaultPricesName))
	default:
		fmt.Fprintln(env.Out, tcRenderText(header, priced, totalTokens, totalCost, sidechainTokens, unpriced, prices, defaultPricesName))
	}
	return nil
}
