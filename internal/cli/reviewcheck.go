package cli

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// review-check — steps 9+10 for a host that has no subagents: two reviews,
// each from a context that has never seen this code being written.
//
// Why this exists. Step 9's value comes from ignorance: the Critic
// disagrees with the author because the Critic was not there when the code
// was argued into existence. On a host with subagents the step gets that
// for free. On a host without them the main agent has no choice but to
// play the Critic itself, in the same session that just wrote the code, and
// the step quietly turns into the author approving the author — a
// degradation nothing in the transcript shows. This calls a model over HTTP
// with nothing in the context but the personas, the diff, the rules and the
// 7a cross-check, and prints what comes back. It is not a host: no edits
// are applied, no commands are run, nothing is written to disk.
//
// Exit 0 means the reviews happened, not that they passed. Exit code 1 is
// not used, on purpose: this does not read what it prints, does not count
// Critical/Important/Nit, and could not issue a verdict — that is the main
// agent's, at the merge step. Exit 2 means a report could not be obtained,
// in whole or in part; whatever did arrive is still printed, and the output
// names the persona that is missing.

const rcKeyEnv = "TRUNKLINE_REVIEW_API_KEY"

type rcPersona struct {
	title      string
	files      []string
	guidelines bool
}

var rcPersonas = map[string]rcPersona{
	"critic": {
		title: "Critic agent (step 9)",
		files: []string{
			"agents/code-reviewer.md",
			"skills/code-review-and-quality/SKILL.md",
			"skills/doubt-driven-development/SKILL.md",
		},
		guidelines: true,
	},
	"security": {
		title: "Security agent (step 10)",
		files: []string{
			"agents/security-auditor.md",
			"skills/security-and-hardening/SKILL.md",
			"references/security-checklist.md",
		},
		guidelines: false,
	},
}

var rcPersonaOrder = []string{"critic", "security"}

const rcMinReportChars = 400
const rcWarnDiffBytes = 400_000

var rcRetryStatus = map[int]bool{408: true, 429: true, 500: true, 502: true, 503: true, 504: true}

func rcDie(env Env, format string, a ...any) error {
	fmt.Fprintf(env.Err, "review-check: "+format+"\n", a...)
	return exitCode(2)
}

func rcReadText(env Env, path, what string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", rcDie(env, "cannot read %s %s: %v", what, path, err)
	}
	return string(b), nil
}

type rcGuideline struct{ role, path string }

// rcPersonaPrompt builds the system prompt: the shipped files, verbatim, in
// the step's order.
func rcPersonaPrompt(env Env, root, name string, guidelines []rcGuideline) (string, error) {
	var parts []string
	for _, rel := range rcPersonas[name].files {
		path := filepath.Join(root, rel)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return "", rcDie(env, "%s is missing. The personas live in the agent-skills "+
				"submodule; if it is empty, run `git submodule update --init`, "+
				"or point --skills-root at the vendored copy "+
				"(.trunkline/agent-skills). This command does not paraphrase a "+
				"missing persona — a review from an invented reviewer is worse "+
				"than no review", path)
		}
		text, err := rcReadText(env, path, "persona file")
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("<<<FILE %s>>>\n%s", rel, text))
	}
	for _, g := range guidelines {
		text, err := rcReadText(env, g.path, "guideline")
		if err != nil {
			return "", err
		}
		parts = append(parts, fmt.Sprintf("<<<PROJECT GUIDELINE role=%s file=%s>>>\n%s", g.role, g.path, text))
	}
	return strings.Join(parts, "\n\n"), nil
}

// rcTaskPrompt builds the user message: exactly the context step 9+10
// requires be passed in — contents, not paths, since an HTTP call cannot
// read a file it was only given the path to.
func rcTaskPrompt(diff, rules, consistency, changeset, note, language string) string {
	var out []string
	if changeset != "" {
		out = append(out, fmt.Sprintf("# Changeset %s", changeset))
	}
	if note != "" {
		out = append(out, fmt.Sprintf("# Scope of this diff\n\n%s", note))
	}
	if rules != "" {
		out = append(out, fmt.Sprintf("# BUSINESS RULES (step 2 registry)\n\n%s", rules))
	}
	if consistency != "" {
		out = append(out, fmt.Sprintf("# Cross-check of step 7a\n\n%s", consistency))
	}
	out = append(out, "# Changeset diff\n\n```diff\n"+strings.TrimRight(diff, "\n")+"\n```")
	instruction := "Review this changeset now, following your persona's report " +
		"template exactly. Output the report and nothing else."
	if language != "" {
		instruction += fmt.Sprintf(" Write the report in %s — the project's own "+
			"language for its code and docs — except for "+
			"`file:symbol` references, code identifiers and code "+
			"excerpts, which stay as they are in the source.", language)
	}
	out = append(out, instruction)
	return strings.Join(out, "\n\n")
}

var rcTokenField = "max_tokens"

// rcPost makes one POST to an OpenAI-compatible /chat/completions, with
// backoff. Never returns a process-ending error: a failed Security call
// must not discard a Critic report that already arrived and was paid for.
func rcPost(url string, payload map[string]any, key string, timeout time.Duration, retries int) (map[string]any, string) {
	last := "unknown error"
	client := &http.Client{Timeout: timeout}
	// A plain for-condition loop, not a C-style one: the max_tokens rename
	// below must retry without spending from the retry budget, i.e. without
	// incrementing attempt, and a for-loop's post-statement would run on
	// every `continue` regardless of which branch took it.
	attempt := 0
	for attempt <= retries {
		body, _ := json.Marshal(payload)
		req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+key)
		resp, err := client.Do(req)
		if err != nil {
			last = fmt.Sprintf("connection failed: %v", err)
		} else {
			raw, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				var data map[string]any
				if jerr := json.Unmarshal(raw, &data); jerr != nil {
					return nil, fmt.Sprintf("HTTP 200 but the body is not JSON: %q", truncate(string(raw), 200))
				}
				return data, ""
			}
			detail := truncate(string(raw), 300)
			last = fmt.Sprintf("HTTP %d: %s", resp.StatusCode, detail)
			if resp.StatusCode == 400 && strings.Contains(detail, "max_completion_tokens") {
				if _, ok := payload["max_tokens"]; ok {
					rcTokenField = "max_completion_tokens"
					payload[rcTokenField] = payload["max_tokens"]
					delete(payload, "max_tokens")
					continue // does not consume the retry budget
				}
			}
			if !rcRetryStatus[resp.StatusCode] {
				return nil, last
			}
		}
		if attempt < retries {
			time.Sleep(time.Duration(2<<attempt) * time.Second)
		}
		attempt++
	}
	return nil, last
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// rcExtract returns (text, finishReason, usageString, complaintOrEmpty).
func rcExtract(data map[string]any) (string, string, string, string) {
	choices, _ := data["choices"].([]any)
	if len(choices) == 0 {
		return "", "", "", fmt.Sprintf("no choices in the response: %s", truncate(fmt.Sprintf("%v", data), 200))
	}
	choice, _ := choices[0].(map[string]any)
	msg, _ := choice["message"].(map[string]any)
	text := strings.TrimSpace(stringOf(msg["content"]))
	finish := stringOf(choice["finish_reason"])
	usage := rcUsageOf(data)
	if text == "" {
		if reasoning := stringOf(msg["reasoning_content"]); reasoning != "" {
			return "", finish, usage, "the reply carries only reasoning_content and no answer — the token " +
				"budget ran out before the report started; raise --max-tokens"
		}
		return "", finish, usage, "the reply is empty"
	}
	return text, finish, usage, ""
}

func rcUsageOf(data map[string]any) string {
	u, _ := data["usage"].(map[string]any)
	if len(u) == 0 {
		return "usage not reported"
	}
	get := func(k string) string {
		if v, ok := u[k]; ok {
			return fmt.Sprintf("%v", int(toFloat(v)))
		}
		return "?"
	}
	return fmt.Sprintf("prompt %s + completion %s = %s tokens", get("prompt_tokens"), get("completion_tokens"), get("total_tokens"))
}

type rcArgs struct {
	baseURL, model, skillsRoot, language string
	maxTokens                            int
	temperature                          *float64
	timeout                              time.Duration
	retries, minChars                    int
}

// rcRunPersona returns (reportTextOrEmpty, provenanceLine, complaintOrEmpty).
func rcRunPersona(args rcArgs, key, system, user string) (string, string, string) {
	payload := map[string]any{
		"model": args.model,
		"messages": []map[string]string{
			{"role": "system", "content": system},
			{"role": "user", "content": user},
		},
		rcTokenField: args.maxTokens,
	}
	if args.temperature != nil {
		payload["temperature"] = *args.temperature
	}
	url := strings.TrimRight(args.baseURL, "/") + "/chat/completions"

	started := time.Now()
	data, errMsg := rcPost(url, payload, key, args.timeout, args.retries)
	elapsed := time.Since(started)
	if errMsg != "" {
		return "", "", errMsg
	}

	text, finish, usage, complaint := rcExtract(data)
	temp := "default"
	if args.temperature != nil {
		temp = fmt.Sprintf("%v", *args.temperature)
	}
	finishDisp := finish
	if finishDisp == "" {
		finishDisp = "not reported"
	}
	prov := fmt.Sprintf("model `%s` via `%s` · temperature %s · max_tokens %d · %s · %.0fs · finish_reason `%s`",
		args.model, args.baseURL, temp, args.maxTokens, usage, elapsed.Seconds(), finishDisp)
	if complaint != "" {
		return "", prov, complaint
	}
	if finish == "length" {
		return text, prov, "the report was cut off by the token limit " +
			"(finish_reason: length) — it is incomplete however " +
			"complete it reads; raise --max-tokens and rerun"
	}
	if len(text) < args.minChars {
		return text, prov, fmt.Sprintf("the reply is %d characters, below the "+
			"%d this command treats as the floor for a "+
			"report — it is an acknowledgement or a refusal, "+
			"not a review", len(text), args.minChars)
	}
	return text, prov, ""
}

func runReviewCheck(env Env, args []string) error {
	fs := flag.NewFlagSet("review-check", flag.ContinueOnError)
	fs.SetOutput(env.Err)
	var baseURL, model, diffPath, rulesPath, consistencyPath, language, skillsRoot, only, changeset, note string
	var guidelineArgs repeatedFlag
	var maxTokens, timeoutSec, retries, minChars int
	var temperature float64
	var dryRun bool
	fs.StringVar(&baseURL, "base-url", os.Getenv("TRUNKLINE_REVIEW_BASE_URL"),
		"OpenAI-compatible API root (env TRUNKLINE_REVIEW_BASE_URL). No default: "+
			"where the diff is sent is stated, never guessed")
	fs.StringVar(&model, "model", os.Getenv("TRUNKLINE_REVIEW_MODEL"), "model id (env TRUNKLINE_REVIEW_MODEL)")
	fs.StringVar(&diffPath, "diff", "", "the changeset diff; read from stdin when omitted")
	fs.StringVar(&rulesPath, "rules", "", "the step 2 rule registry")
	fs.StringVar(&consistencyPath, "consistency", "", "this changeset's step 7a cross-check")
	fs.Var(&guidelineArgs, "guideline", "a profile guideline the Critic checks conformance to; repeatable")
	fs.StringVar(&language, "language", os.Getenv("TRUNKLINE_REVIEW_LANGUAGE"),
		"the project's `language` flag (env TRUNKLINE_REVIEW_LANGUAGE)")
	fs.StringVar(&skillsRoot, "skills-root", "agent-skills", "where the personas and skills live")
	fs.StringVar(&only, "only", "", "run one persona: critic or security")
	fs.StringVar(&changeset, "changeset", "", "changeset number, for the header")
	fs.StringVar(&note, "note", "", "what produced this diff and what it was trying to achieve")
	fs.IntVar(&maxTokens, "max-tokens", 8000, "")
	fs.Float64Var(&temperature, "temperature", 0.0, "pass a negative value to omit the field entirely")
	fs.IntVar(&timeoutSec, "timeout", 600, "")
	fs.IntVar(&retries, "retries", 2, "retries on 429 and 5xx, with backoff")
	fs.IntVar(&minChars, "min-chars", rcMinReportChars, "shorter replies are reported as non-answers")
	fs.BoolVar(&dryRun, "dry-run", false, "assemble the prompts and print their sizes; make no call")
	fs.Usage = func() {
		fmt.Fprint(env.Err, "Usage: trunkline review-check --base-url URL --model NAME [flags]\n\n"+
			"Steps 9+10 over HTTP, for a host with no subagents. The diff is read\n"+
			"from --diff or stdin. See core/developer/steps/09-10-critic-security.md.\n")
	}
	if err := fs.Parse(args); err != nil {
		return exitCode(2)
	}
	if only != "" {
		if _, ok := rcPersonas[only]; !ok {
			return rcDie(env, "--only must be critic or security")
		}
	}

	var temp *float64
	if temperature >= 0 {
		temp = &temperature
	}

	var guidelines []rcGuideline
	for _, spec := range guidelineArgs {
		role, path, ok := strings.Cut(spec, "=")
		if !ok {
			return rcDie(env, "--guideline expects ROLE=PATH, got %q", spec)
		}
		path = strings.TrimSpace(path)
		if info, err := os.Stat(path); err != nil || info.IsDir() {
			return rcDie(env, "--guideline %s: %s is not a file", spec, path)
		}
		guidelines = append(guidelines, rcGuideline{strings.TrimSpace(role), path})
	}

	var diff string
	if diffPath != "" {
		d, err := rcReadText(env, diffPath, "diff")
		if err != nil {
			return err
		}
		diff = d
	} else if stat, _ := os.Stdin.Stat(); stat != nil && (stat.Mode()&os.ModeCharDevice) == 0 {
		b, _ := io.ReadAll(os.Stdin)
		diff = string(b)
	} else {
		return rcDie(env, "no diff: pass --diff FILE or pipe one in. A review of an empty diff is "+
			"a report about nothing, and it reads exactly like a report about "+
			"something")
	}
	if strings.TrimSpace(diff) == "" {
		return rcDie(env, "the diff is empty. Either the changeset has no changes — in which case "+
			"there is nothing for step 9+10 to review — or the wrong range was "+
			"diffed; both are the step's problem to fix, not this command's to paper "+
			"over")
	}
	if len(diff) > rcWarnDiffBytes {
		fmt.Fprintf(env.Err, "NOTE     the diff is %d KB. It is sent "+
			"whole and unsplit: a reviewer\n         shown half a changeset cannot "+
			"find the defect that lives between the halves.\n         If this "+
			"exceeds the model's window the call will fail loudly, which is the\n"+
			"         intended outcome — step 4 sized the changeset wrong.\n", len(diff)/1024)
	}

	var rules, consistency string
	if rulesPath != "" {
		r, err := rcReadText(env, rulesPath, "rule registry")
		if err != nil {
			return err
		}
		rules = r
	}
	if consistencyPath != "" {
		c, err := rcReadText(env, consistencyPath, "7a cross-check")
		if err != nil {
			return err
		}
		consistency = c
	}
	if rules == "" {
		fmt.Fprint(env.Err, "NOTE     no --rules given. Legitimate on the `bug` and `refactor` routes, "+
			"where step 2\n         is not in the route and there are no identifiers; "+
			"elsewhere the Critic will\n         review the code against nothing but "+
			"itself.\n")
	}
	if consistency == "" {
		fmt.Fprint(env.Err, "NOTE     no --consistency given: the reviewers will not see what step 7a "+
			"found.\n")
	}
	if language == "" {
		fmt.Fprint(env.Err, "NOTE     no --language given: both reports will come back in English, "+
			"whatever the\n         project's `language` flag says — the personas "+
			"carry no other signal.\n")
	}

	names := rcPersonaOrder
	if only != "" {
		names = []string{only}
	}
	user := rcTaskPrompt(diff, rules, consistency, changeset, note, language)
	prompts := map[string]string{}
	for _, n := range names {
		var g []rcGuideline
		if rcPersonas[n].guidelines {
			g = guidelines
		}
		p, err := rcPersonaPrompt(env, skillsRoot, n, g)
		if err != nil {
			return err
		}
		prompts[n] = p
	}

	if dryRun {
		for _, n := range names {
			total := len(prompts[n]) + len(user)
			fmt.Fprintf(env.Out, "%s: system %s chars + task %s chars = %s (~%sK tokens)\n",
				rcPersonas[n].title, tcN(len(prompts[n])), tcN(len(user)), tcN(total), tcN(total/3600))
		}
		fmt.Fprint(env.Out, "\nREVIEW-CHECK: dry run — prompts assembled, no call made, nothing "+
			"reviewed.\n")
		return nil
	}

	key := os.Getenv(rcKeyEnv)
	if key == "" {
		return rcDie(env, "%s is not set. The key is taken from the environment and has no "+
			"flag: this command prints its own invocation into a report that ends up "+
			"in .pipeline/workdir/review/, and a key on the command line would be committed "+
			"with it", rcKeyEnv)
	}
	if baseURL == "" {
		return rcDie(env, "--base-url is required (or TRUNKLINE_REVIEW_BASE_URL). There is no "+
			"default because the default would decide, silently, where this "+
			"project's source code is sent")
	}
	if model == "" {
		return rcDie(env, "--model is required (or TRUNKLINE_REVIEW_MODEL)")
	}

	rcArgsV := rcArgs{baseURL, model, skillsRoot, language, maxTokens, temp,
		time.Duration(timeoutSec) * time.Second, retries, minChars}

	type result struct {
		name, text, prov, complaint string
	}
	var results []result
	var failures []string
	for _, n := range names {
		text, prov, complaint := rcRunPersona(rcArgsV, key, prompts[n], user)
		results = append(results, result{n, text, prov, complaint})
		if complaint != "" {
			failures = append(failures, n)
		}
	}

	label := ""
	if changeset != "" {
		label = fmt.Sprintf(" — changeset %s", changeset)
	}
	fmt.Fprint(env.Out, "<!-- Obtained by `trunkline review-check`: the host declares "+
		"no C1, so steps 9+10 ran as two API calls instead of two subagents. Each "+
		"call started from a clean context. -->\n\n")
	for _, r := range results {
		fmt.Fprintf(env.Out, "## %s%s\n\n", rcPersonas[r.name].title, label)
		if r.prov != "" {
			fmt.Fprintf(env.Out, "*%s*\n\n", r.prov)
		}
		if r.complaint != "" {
			fmt.Fprintf(env.Out, "**NOT A REPORT — %s**\n\n", r.complaint)
		}
		if r.text != "" {
			fmt.Fprintf(env.Out, "%s\n\n", strings.TrimSpace(r.text))
		} else if r.complaint == "" {
			fmt.Fprint(env.Out, "*(no text returned)*\n\n")
		}
	}

	var ok []string
	for _, r := range results {
		if r.complaint == "" {
			ok = append(ok, r.name)
		}
	}
	okSet := map[string]bool{}
	for _, n := range ok {
		okSet[n] = true
	}
	var statusParts []string
	for _, n := range names {
		status := "FAILED"
		if okSet[n] {
			status = "ok"
		}
		statusParts = append(statusParts, fmt.Sprintf("%s: %s", n, status))
	}
	completeness := "COMPLETE"
	if len(failures) > 0 {
		completeness = "INCOMPLETE"
	}
	fmt.Fprintf(env.Out, "REVIEW-CHECK: %d/%d persona(s) — %s — %s\n",
		len(ok), len(names), strings.Join(statusParts, ", "), completeness)
	if len(failures) > 0 {
		var missing []string
		for _, n := range failures {
			missing = append(missing, rcPersonas[n].title)
		}
		fmt.Fprintf(env.Out, "Missing: %s. This is "+
			"not a completed step 9+10. Record `review_verify: unavailable` in the "+
			"journal, fall back to playing both roles in sequence in the session, "+
			"and say out loud that the pass was degraded — never that the review "+
			"was done.\n", strings.Join(missing, ", "))
	}
	fmt.Fprint(env.Out, "No verdict: this command does not read what it printed. Counting Critical / "+
		"Important / Nit and Critical / High / Medium-Low, deciding the rollback to "+
		"step 8, and writing .pipeline/workdir/review/ are the main agent's, at the merge "+
		"step — there is exactly one writer.\n")

	if len(failures) > 0 {
		return exitCode(2)
	}
	return nil
}
