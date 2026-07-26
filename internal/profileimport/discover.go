package profileimport

import (
	"os"
	"path/filepath"

	"github.com/flobilosaurus/agent-env/internal/config"
	"github.com/flobilosaurus/agent-env/internal/paths"
)

type SourceKind string

const (
	SourceKindHome    SourceKind = "home"
	SourceKindProfile SourceKind = "profile"
)

type Source struct {
	ID    string
	Label string
	Path  string
	Kind  SourceKind
}

type Intent struct {
	Source   Source
	GroupIDs []string
}

func ProfileSources(p paths.Paths, profiles []config.Profile, originalHome string) []Source {
	var sources []Source
	seen := map[string]bool{}
	add := func(s Source) {
		root, ok := existingDirRoot(s.Path)
		if !ok {
			return
		}
		if seen[root] {
			return
		}
		seen[root] = true
		s.Path = root
		sources = append(sources, s)
	}
	if originalHome != "" {
		add(Source{ID: "home", Label: "Original HOME", Path: originalHome, Kind: SourceKindHome})
	}
	for _, profile := range profiles {
		add(Source{ID: "profile:" + profile.Name, Label: "profile: " + profile.Name, Path: p.ProfileHome(profile.Name), Kind: SourceKindProfile})
	}
	return sources
}

func AvailableGroups(source Source, groups []Group) []Group {
	available := make([]Group, 0, len(groups))
	for _, group := range groups {
		path := filepath.Join(source.Path, group.SourceRel)
		info, err := os.Stat(path)
		if err != nil {
			continue
		}
		switch group.Kind {
		case KindFile:
			if info.Mode().IsRegular() {
				available = append(available, group)
			}
		case KindDir:
			if info.IsDir() {
				available = append(available, group)
			}
		}
	}
	return available
}

func existingDirRoot(path string) (string, bool) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", false
	}
	abs = filepath.Clean(abs)
	info, err := os.Stat(abs)
	if err != nil || !info.IsDir() {
		return "", false
	}
	return abs, true
}
