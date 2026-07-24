package profileimport

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type PathResult struct {
	GroupID string
	Path    string
}

type Result struct {
	Copied  []PathResult
	Skipped []PathResult
}

func ImportSelection(targetHome string, intent Intent, catalog []Group) (Result, error) {
	var result Result
	if _, ok := existingDirRoot(intent.Source.Path); !ok {
		return result, fmt.Errorf("source %q does not exist or is not a directory: %s", intent.Source.Label, intent.Source.Path)
	}
	groups := map[string]Group{}
	for _, group := range catalog {
		groups[group.ID] = group
	}
	for _, id := range intent.GroupIDs {
		group, ok := groups[id]
		if !ok {
			return result, fmt.Errorf("unknown import group %q", id)
		}
		src := filepath.Join(intent.Source.Path, group.SourceRel)
		dst := filepath.Join(targetHome, group.TargetRel)
		if _, err := os.Lstat(dst); err == nil {
			result.Skipped = append(result.Skipped, PathResult{GroupID: group.ID, Path: group.TargetRel})
			continue
		} else if !os.IsNotExist(err) {
			return result, fmt.Errorf("import %s %s: stat target %s: %w", group.Agent, group.Label, dst, err)
		}
		if err := validateSourceKind(src, group.Kind); err != nil {
			return result, fmt.Errorf("import %s %s: %w", group.Agent, group.Label, err)
		}
		if err := os.MkdirAll(filepath.Dir(dst), 0o700); err != nil {
			return result, fmt.Errorf("import %s %s: create parent %s: %w", group.Agent, group.Label, filepath.Dir(dst), err)
		}
		if err := copyPath(src, dst); err != nil {
			return result, fmt.Errorf("import %s %s: copy %s to %s: %w", group.Agent, group.Label, src, dst, err)
		}
		result.Copied = append(result.Copied, PathResult{GroupID: group.ID, Path: group.TargetRel})
	}
	return result, nil
}

func validateSourceKind(path string, kind Kind) error {
	info, err := os.Lstat(path)
	if err != nil {
		return fmt.Errorf("source %s: %w", path, err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("source %s is a symlink, expected %s", path, kind)
	}
	if kind == KindFile && !info.Mode().IsRegular() {
		return fmt.Errorf("source %s is not a regular file", path)
	}
	if kind == KindDir && !info.IsDir() {
		return fmt.Errorf("source %s is not a directory", path)
	}
	return nil
}

func copyPath(src, dst string) error {
	info, err := os.Lstat(src)
	if err != nil {
		return err
	}
	if info.Mode()&os.ModeSymlink != 0 {
		link, err := os.Readlink(src)
		if err != nil {
			return err
		}
		return os.Symlink(link, dst)
	}
	if info.IsDir() {
		return copyDir(src, dst, info.Mode().Perm())
	}
	if info.Mode().IsRegular() {
		return copyFile(src, dst, info.Mode().Perm())
	}
	return fmt.Errorf("unsupported file type %s", src)
}

func copyDir(src, dst string, mode os.FileMode) error {
	if err := os.Mkdir(dst, mode); err != nil {
		return err
	}
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if err := copyPath(filepath.Join(src, entry.Name()), filepath.Join(dst, entry.Name())); err != nil {
			return err
		}
	}
	return os.Chmod(dst, mode)
}

func copyFile(src, dst string, mode os.FileMode) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Chmod(dst, mode)
}
