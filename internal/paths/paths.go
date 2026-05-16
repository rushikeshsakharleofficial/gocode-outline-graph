package paths

import (
	"fmt"
	"os"
	"path/filepath"
)

const (
	DBDir  = ".code-outline-graph"
	DBFile = "index.db"
	EnvVar = "CODE_OUTLINE_PROJECT"
)

// ResolveProjectPath returns an absolute path for the project root.
// Resolution order:
//  1. CODE_OUTLINE_PROJECT env var
//  2. projectPath argument (if non-empty)
//  3. Current working directory
func ResolveProjectPath(projectPath string) (string, error) {
	if env := os.Getenv(EnvVar); env != "" {
		abs, err := filepath.Abs(env)
		if err != nil {
			return "", fmt.Errorf("resolving CODE_OUTLINE_PROJECT %q: %w", env, err)
		}
		return abs, nil
	}
	if projectPath != "" {
		abs, err := filepath.Abs(projectPath)
		if err != nil {
			return "", fmt.Errorf("resolving project path %q: %w", projectPath, err)
		}
		return abs, nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("getting working directory: %w", err)
	}
	return cwd, nil
}

// ProjectDBPath returns the path to the SQLite index for a project root.
func ProjectDBPath(projectPath string) string {
	return filepath.Join(projectPath, DBDir, DBFile)
}

// EnsureProjectDBDir creates .code-outline-graph inside the project root.
func EnsureProjectDBDir(projectPath string) error {
	dir := filepath.Join(projectPath, DBDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating project DB directory %q: %w", dir, err)
	}
	return nil
}
