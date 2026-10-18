package install

import (
	"fmt"
	"path"
	"path/filepath"

	"github.com/vseminoy/trunkline/internal/state"
)

// safeJoin resolves a project-relative path against dir, refusing anything that
// would land outside the project.
//
// It exists because not every path trunkline acts on comes from the binary.
// .trunkline/state.yaml is committed by design — `update` diffs against it — so
// its list of files travels with the repository and can be edited by anyone who
// can open a pull request. A `..` in that list is not a stale entry to clean up;
// it is an instruction to delete or overwrite something outside the project.
func safeJoin(dir, rel string) (string, error) {
	if !state.ValidRel(rel) {
		return "", fmt.Errorf("%q is not a path inside the project", rel)
	}
	return filepath.Join(dir, filepath.FromSlash(path.Clean(rel))), nil
}
