// Package trunkline exposes the pipeline payload that the CLI installs into a
// project: the stack-agnostic core, the stack profiles and the vendored parts
// of the agent-skills submodule the pipeline references by path.
//
// Nothing here knows about a target project — see internal/install.
package trunkline

import (
	"embed"
	"strings"
)

// Core is the stack-agnostic pipeline: routes, gates, traceability, knowledge
// base format, telemetry. Rooted at "core/".
//
// It is not markdown only. core/developer/scripts/ still carries the gates that
// have to execute inside the target project rather than in this repository —
// unlike the linters under scripts/, which police this repository and are
// never vendored. The step 7a trace check, the review-check verifier, the
// token-cost report and the step 21 knowledge-base sync are no longer files
// here: they are `trunkline trace-check` / `review-check` / `token-cost` /
// `kb-sync`, subcommands of this same binary (internal/cli), so a project
// only needs the binary on PATH, not a python3 to go with it. prices.json
// stays here as data `token-cost` embeds and `--prices` can override; the
// approval gate stays a real file, because it runs as a git hook and has to
// travel into the project as a script, not as a binary subcommand.
//
//go:embed core
var Core embed.FS

// Profiles holds every stack profile shipped with the binary. Rooted at
// "profiles/". A profile is a directory with a profile.yaml manifest.
//
//go:embed profiles
var Profiles embed.FS

// Publishers holds the contracts of the task publishers a profile may declare:
// what a tool requires of the files it consumes. Rooted at "publishers/".
//
// A contract belongs to its tool, not to a profile — several profiles can declare
// the same publisher, and each keeping its own copy of the parsing rules is how
// those copies drift out of step with the code. It ships because the analyst has
// to obey it while working inside a project, where the tool's own repository is
// not present.
//
//go:embed publishers
var Publishers embed.FS

// Skills is the subset of the agent-skills submodule the pipeline references by
// path: hooks (doc cache, simplify-ignore), the shared reference checklists, the
// skill files that steps load, and the personas steps 7, 9 and 10 take their role
// and report format from. MIT, Addy Osmani — LICENSE travels with it.
//
// agents/ is not optional: without it the Critic, the Security auditor and the
// Tester have a `**Persona:**` line pointing at a file that is not there, and the
// step silently loses both its role and its report format.
//
//go:embed all:agent-skills/agents
//go:embed all:agent-skills/hooks
//go:embed all:agent-skills/references
//go:embed all:agent-skills/skills
//go:embed agent-skills/LICENSE
var Skills embed.FS

//go:embed VERSION
var versionRaw string

// Version is the semver of the core shipped in this binary. Profiles carry
// their own version in profile.yaml and move independently.
func Version() string {
	return strings.TrimSpace(versionRaw)
}
