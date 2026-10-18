package git

import (
	"errors"
	"path/filepath"
	"strings"
)

func (CLI) ReviewDiff(path, base, file string) (string, error) {
	if file == "" || filepath.IsAbs(file) || strings.HasPrefix(filepath.Clean(file), "..") || base == "" || strings.HasPrefix(base, "-") {
		return "", errors.New("invalid review file or base")
	}
	if _, err := run(path, "rev-parse", "--verify", base+"^{commit}"); err != nil {
		return "", errors.New("review base is unavailable locally; fetch the base branch in Terminal")
	}
	return run(path, "diff", "--no-ext-diff", "--no-textconv", base, "--", file)
}
