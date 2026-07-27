package profileimport

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/flobilosaurus/agent-env/internal/config"
	"github.com/flobilosaurus/agent-env/internal/paths"
)

func TestCatalogContainsExpectedGroupsAndStableIDs(t *testing.T) {
	groups := Catalog()
	if len(groups) != 39 {
		t.Fatalf("group count=%d", len(groups))
	}
	ids := make([]string, len(groups))
	for i, group := range groups {
		ids[i] = group.ID
	}
	wantPrefix := []string{"pi.agents-md", "pi.auth", "pi.settings", "pi.models", "pi.trust", "pi.keybindings", "pi.extensions", "pi.skills"}
	if !reflect.DeepEqual(ids[:len(wantPrefix)], wantPrefix) {
		t.Fatalf("unstable prefix: %+v", ids[:len(wantPrefix)])
	}
	for _, want := range []string{"claude.state", "claude.config-state", "claude.credentials", "claude.agents", "opencode.auth", "opencode.state", "codex.config", "codex.skills"} {
		if !contains(ids, want) {
			t.Fatalf("missing %s in %+v", want, ids)
		}
	}
}

func TestAvailableGroupsFiltersExistingMatchingKind(t *testing.T) {
	home := t.TempDir()
	writeFile(t, filepath.Join(home, ".pi/agent/auth.json"), "auth")
	mkdir(t, filepath.Join(home, ".pi/agent/skills"))
	writeFile(t, filepath.Join(home, ".claude/agents"), "not-dir")
	mkdir(t, filepath.Join(home, ".codex/config.toml"))
	got := AvailableGroups(Source{Path: home}, Catalog())
	ids := groupIDs(got)
	want := []string{"pi.auth", "pi.skills"}
	if !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%+v want=%+v", ids, want)
	}
}

func TestAvailableGroupsFollowsTopLevelSymlinks(t *testing.T) {
	home, targets := t.TempDir(), t.TempDir()
	mkdir(t, filepath.Join(home, ".pi/agent"))
	writeFile(t, filepath.Join(targets, "settings.json"), "settings")
	mkdir(t, filepath.Join(targets, "skills"))
	if err := os.Symlink(filepath.Join(targets, "settings.json"), filepath.Join(home, ".pi/agent/settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(targets, "skills"), filepath.Join(home, ".pi/agent/skills")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("missing", filepath.Join(home, ".pi/agent/auth.json")); err != nil {
		t.Fatal(err)
	}
	got := AvailableGroups(Source{Path: home}, Catalog())
	want := []string{"pi.settings", "pi.skills"}
	if ids := groupIDs(got); !reflect.DeepEqual(ids, want) {
		t.Fatalf("ids=%+v want=%+v", ids, want)
	}
}

func TestProfileSourcesHidesMissingAndKeepsZeroImportableExistingRoots(t *testing.T) {
	data := t.TempDir()
	home := t.TempDir()
	mkdir(t, filepath.Join(data, "profiles/existing/home"))
	p := paths.Paths{DataRoot: data}
	got := ProfileSources(p, []config.Profile{{Name: "existing"}, {Name: "missing"}}, home)
	if len(got) != 2 {
		t.Fatalf("sources=%+v", got)
	}
	if got[0].Label != "Original HOME" || got[1].Label != "profile: existing" {
		t.Fatalf("wrong order/labels: %+v", got)
	}
}

func TestProfileSourcesDeduplicatesEqualRoots(t *testing.T) {
	data := t.TempDir()
	root := filepath.Join(data, "profiles/p/home")
	mkdir(t, root)
	got := ProfileSources(paths.Paths{DataRoot: data}, []config.Profile{{Name: "p"}}, root)
	if len(got) != 1 || got[0].Kind != SourceKindHome {
		t.Fatalf("dedupe failed: %+v", got)
	}
}

func TestImportSelectionCopiesFileCreatingParents(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, ".pi/agent/auth.json"), "secret")
	res, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"pi.auth"}}, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dst, ".pi/agent/auth.json"), "secret")
	if !reflect.DeepEqual(groupPathIDs(res.Copied), []string{"pi.auth:.pi/agent/auth.json"}) {
		t.Fatalf("copied=%+v", res.Copied)
	}
}

func TestImportSelectionMapsLegacyClaudeStateIntoConfigDir(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, ".claude.json"), `{"theme":"dark"}`)
	_, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"claude.state"}}, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dst, ".claude/.claude.json"), `{"theme":"dark"}`)
}

func TestImportSelectionCopiesClaudeConfigDirState(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, ".claude/.claude.json"), `{"theme":"light"}`)
	_, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"claude.config-state"}}, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dst, ".claude/.claude.json"), `{"theme":"light"}`)
}

func TestImportSelectionCopiesDirectoryRecursively(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, ".pi/agent/skills/a/SKILL.md"), "skill")
	res, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"pi.skills"}}, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dst, ".pi/agent/skills/a/SKILL.md"), "skill")
	if len(res.Copied) != 1 || len(res.Skipped) != 0 {
		t.Fatalf("result=%+v", res)
	}
}

func TestImportSelectionDereferencesTopLevelSymlinks(t *testing.T) {
	src, dst, targets := t.TempDir(), t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(targets, "settings.json"), "settings")
	writeFile(t, filepath.Join(targets, "skills/a/SKILL.md"), "skill")
	mkdir(t, filepath.Join(src, ".pi/agent"))
	if err := os.Symlink(filepath.Join(targets, "settings.json"), filepath.Join(src, ".pi/agent/settings.json")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(targets, "skills"), filepath.Join(src, ".pi/agent/skills")); err != nil {
		t.Fatal(err)
	}
	_, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"pi.settings", "pi.skills"}}, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dst, ".pi/agent/settings.json"), "settings")
	assertFile(t, filepath.Join(dst, ".pi/agent/skills/a/SKILL.md"), "skill")
	assertNotSymlink(t, filepath.Join(dst, ".pi/agent/settings.json"))
	assertNotSymlink(t, filepath.Join(dst, ".pi/agent/skills"))
}

func TestImportSelectionDereferencesNestedSymlinks(t *testing.T) {
	src, dst, targets := t.TempDir(), t.TempDir(), t.TempDir()
	mkdir(t, filepath.Join(src, ".pi/agent/skills"))
	writeFile(t, filepath.Join(targets, "prompt.md"), "prompt")
	writeFile(t, filepath.Join(targets, "bundle/SKILL.md"), "bundle")
	if err := os.Symlink(filepath.Join(targets, "prompt.md"), filepath.Join(src, ".pi/agent/skills/prompt.md")); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filepath.Join(targets, "bundle"), filepath.Join(src, ".pi/agent/skills/bundle")); err != nil {
		t.Fatal(err)
	}
	if _, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"pi.skills"}}, Catalog()); err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dst, ".pi/agent/skills/prompt.md"), "prompt")
	assertFile(t, filepath.Join(dst, ".pi/agent/skills/bundle/SKILL.md"), "bundle")
	assertNotSymlink(t, filepath.Join(dst, ".pi/agent/skills/prompt.md"))
	assertNotSymlink(t, filepath.Join(dst, ".pi/agent/skills/bundle"))
}

func TestImportSelectionRejectsDirectorySymlinkCycles(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	skills := filepath.Join(src, ".pi/agent/skills")
	mkdir(t, skills)
	if err := os.Symlink(skills, filepath.Join(skills, "loop")); err != nil {
		t.Fatal(err)
	}
	_, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"pi.skills"}}, Catalog())
	if err == nil || !strings.Contains(err.Error(), "source directory cycle") {
		t.Fatalf("err=%v", err)
	}
}

func TestImportSelectionSkipsExistingTarget(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, ".pi/agent/auth.json"), "source")
	writeFile(t, filepath.Join(dst, ".pi/agent/auth.json"), "target")
	res, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"pi.auth"}}, Catalog())
	if err != nil {
		t.Fatal(err)
	}
	assertFile(t, filepath.Join(dst, ".pi/agent/auth.json"), "target")
	if !reflect.DeepEqual(groupPathIDs(res.Skipped), []string{"pi.auth:.pi/agent/auth.json"}) {
		t.Fatalf("skipped=%+v", res.Skipped)
	}
}

func TestImportSelectionStopsOnUnexpectedCopyErrorWithoutRollback(t *testing.T) {
	src, dst := t.TempDir(), t.TempDir()
	writeFile(t, filepath.Join(src, "a"), "ok")
	writeFile(t, filepath.Join(src, "b"), "fail")
	catalog := []Group{
		{ID: "ok", Agent: "test", Label: "ok", SourceRel: "a", TargetRel: "parent", Kind: KindFile},
		{ID: "fail", Agent: "test", Label: "fail", SourceRel: "b", TargetRel: "parent/child", Kind: KindFile},
	}
	res, err := ImportSelection(dst, Intent{Source: Source{Label: "src", Path: src}, GroupIDs: []string{"ok", "fail"}}, catalog)
	if err == nil || !strings.Contains(err.Error(), "parent/child") {
		t.Fatalf("err=%v", err)
	}
	if !reflect.DeepEqual(groupPathIDs(res.Copied), []string{"ok:parent"}) {
		t.Fatalf("copied=%+v", res.Copied)
	}
	assertFile(t, filepath.Join(dst, "parent"), "ok")
}

func TestImportSelectionUnknownGroupID(t *testing.T) {
	_, err := ImportSelection(t.TempDir(), Intent{Source: Source{Label: "src", Path: t.TempDir()}, GroupIDs: []string{"nope"}}, Catalog())
	if err == nil || !strings.Contains(err.Error(), "unknown import group") {
		t.Fatalf("err=%v", err)
	}
}

func TestImportSelectionMissingSourceRoot(t *testing.T) {
	_, err := ImportSelection(t.TempDir(), Intent{Source: Source{Label: "src", Path: filepath.Join(t.TempDir(), "missing")}, GroupIDs: []string{"pi.auth"}}, Catalog())
	if err == nil || !strings.Contains(err.Error(), "source \"src\" does not exist") {
		t.Fatalf("err=%v", err)
	}
}

func contains(items []string, want string) bool {
	for _, item := range items {
		if item == want {
			return true
		}
	}
	return false
}

func groupIDs(groups []Group) []string {
	ids := make([]string, len(groups))
	for i, group := range groups {
		ids[i] = group.ID
	}
	return ids
}

func groupPathIDs(paths []PathResult) []string {
	ids := make([]string, len(paths))
	for i, p := range paths {
		ids[i] = p.GroupID + ":" + p.Path
	}
	return ids
}

func mkdir(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdir(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertFile(t *testing.T, path, content string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("%s=%q want %q", path, got, content)
	}
}

func assertNotSymlink(t *testing.T, path string) {
	t.Helper()
	info, err := os.Lstat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode()&os.ModeSymlink != 0 {
		t.Fatalf("%s is a symlink", path)
	}
}
