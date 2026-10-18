package markdown

import (
	"fmt"
	"path/filepath"
	"regexp"
	"strings"
)

// imageRe matches markdown image syntax: ![alt](target).
var imageRe = regexp.MustCompile(`!\[[^\]]*\]\(([^)]+)\)`)

// PreparedDescription is the result of converting a task body into a JIRA-ready
// description (docs/task-format.md §4.2, TZ §7).
type PreparedDescription struct {
	// Text is the body with local markdown images rewritten to JIRA wiki
	// image syntax (![alt](dir/pic.png) -> !pic.png!).
	Text string
	// Images lists the local image files (paths relative to the task folder)
	// referenced by the body. They must be uploaded as attachments.
	Images []string
}

// isExternal reports whether an image target is an external URL (left as-is,
// not uploaded).
func isExternal(target string) bool {
	t := strings.ToLower(strings.TrimSpace(target))
	return strings.HasPrefix(t, "http://") || strings.HasPrefix(t, "https://")
}

// PrepareDescription rewrites local markdown images in the body to JIRA wiki
// syntax and collects the referenced local image files.
//
// JIRA addresses an attachment by file name, not path, so image base names must
// be unique within a task; a collision is reported as an error.
func PrepareDescription(body string) (PreparedDescription, error) {
	seen := map[string]string{} // base name -> original relative path
	var images []string
	var collisionErr error

	out := imageRe.ReplaceAllStringFunc(body, func(match string) string {
		sub := imageRe.FindStringSubmatch(match)
		target := strings.TrimSpace(sub[1])
		if isExternal(target) {
			return match // external images stay as a plain markdown link
		}
		base := filepath.Base(target)
		if prev, ok := seen[base]; ok && prev != target {
			if collisionErr == nil {
				collisionErr = fmt.Errorf("duplicate image name %q (from %q and %q): image names must be unique within a task", base, prev, target)
			}
			return match
		}
		if _, ok := seen[base]; !ok {
			seen[base] = target
			images = append(images, target)
		}
		return "!" + base + "!"
	})

	if collisionErr != nil {
		return PreparedDescription{}, collisionErr
	}
	return PreparedDescription{Text: out, Images: images}, nil
}
