package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flobilosaurus/agent-env/internal/config"
	"github.com/flobilosaurus/agent-env/internal/profileimport"
	"github.com/flobilosaurus/agent-env/internal/tui"
)

type fakePrompter struct {
	profile      string
	create       bool
	importIntent *profileimport.Intent
	calls        int
	sources      []profileimport.Source
}

func (f *fakePrompter) ChooseProfile(agent string, profiles []config.Profile, sources []profileimport.Source, groups []profileimport.Group) (tui.ProfileChoice, error) {
	f.calls++
	f.sources = append([]profileimport.Source(nil), sources...)
	if f.profile == "" {
		return tui.ProfileChoice{Profile: "new-profile", Create: true, Import: f.importIntent}, nil
	}
	return tui.ProfileChoice{Profile: f.profile, Create: f.create, Import: f.importIntent}, nil
}

type fakeRemovePrompter struct {
	profile string
	calls   int
}

func (f *fakeRemovePrompter) ChooseProfileToRemove(profiles []config.Profile) (string, error) {
	f.calls++
	return f.profile, nil
}

func TestMissingArgs(t *testing.T) {
	var err bytes.Buffer
	code := App{Err: &err}.Run(nil)
	if code == 0 || err.String() == "" {
		t.Fatalf("expected usage error")
	}
}

func TestRunMissingMappingWithFakePrompterCreatesProfileAndMapping(t *testing.T) {
	cfgHome, dataHome, realBin, project, record := setupRunTest(t)
	prompter := &fakePrompter{}
	var out, errBuf bytes.Buffer
	code := App{Out: &out, Err: &errBuf, Prompter: prompter}.Run([]string{"run", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	cfg := loadTestConfig(t, cfgHome)
	key, _ := config.NormalizeProjectPath(project)
	if cfg.Projects[key] != "new-profile" || !cfg.HasProfile("new-profile") {
		t.Fatalf("mapping/profile not saved: %+v", cfg)
	}
	got, _ := os.ReadFile(record)
	if !strings.Contains(string(got), filepath.Join(dataHome, "profiles", "new-profile", "home")) {
		t.Fatalf("wrong home: %s", got)
	}
	_ = realBin
}

func TestRunForceSelectReplacesMappingAndLaunchesWithSelectedProfile(t *testing.T) {
	cfgHome, dataHome, _, project, record := setupRunTest(t)
	writeTestConfig(t, cfgHome, project, "old-profile", "new-profile")
	prompter := &fakePrompter{profile: "new-profile"}
	var out, errBuf bytes.Buffer
	code := App{Out: &out, Err: &errBuf, Prompter: prompter}.Run([]string{"run", "--select", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	if prompter.calls != 1 {
		t.Fatalf("prompter calls=%d", prompter.calls)
	}
	cfg := loadTestConfig(t, cfgHome)
	key, _ := config.NormalizeProjectPath(project)
	if cfg.Projects[key] != "new-profile" {
		t.Fatalf("mapping not replaced: %+v", cfg.Projects)
	}
	got, _ := os.ReadFile(record)
	wantHome := filepath.Join(dataHome, "profiles", "new-profile", "home")
	if !strings.Contains(string(got), "HOME="+wantHome) {
		t.Fatalf("wrong launch home: %s", got)
	}
}

func TestRunClaudeSetsClaudeConfigDirToProfileHome(t *testing.T) {
	cfgHome, dataHome, realBin, project, record := setupRunTest(t)
	writeTestConfig(t, cfgHome, project, "work")
	if err := os.WriteFile(filepath.Join(realBin, "claude"), []byte("#!/bin/sh\nprintf 'HOME=%s\nCLAUDE_CONFIG_DIR=%s\n' \"$HOME\" \"$CLAUDE_CONFIG_DIR\" > \""+record+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf}.Run([]string{"run", "claude"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	got, _ := os.ReadFile(record)
	home := filepath.Join(dataHome, "profiles", "work", "home")
	if !strings.Contains(string(got), "HOME="+home) || !strings.Contains(string(got), "CLAUDE_CONFIG_DIR="+filepath.Join(home, ".claude")) {
		t.Fatalf("wrong env: %s", got)
	}
}

func TestRunSetsXDGDirsToProfileHomeForAllAgents(t *testing.T) {
	cfgHome, dataHome, realBin, project, record := setupRunTest(t)
	writeTestConfig(t, cfgHome, project, "work")
	if err := os.WriteFile(filepath.Join(realBin, "codex"), []byte("#!/bin/sh\nprintf 'HOME=%s\nXDG_CONFIG_HOME=%s\nXDG_DATA_HOME=%s\nXDG_STATE_HOME=%s\n' \"$HOME\" \"$XDG_CONFIG_HOME\" \"$XDG_DATA_HOME\" \"$XDG_STATE_HOME\" > \""+record+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("XDG_CONFIG_HOME", "/host/config")
	t.Setenv("XDG_DATA_HOME", "/host/data")
	t.Setenv("XDG_STATE_HOME", "/host/state")
	var errBuf bytes.Buffer
	code := App{Err: &errBuf}.Run([]string{"run", "codex"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	got, _ := os.ReadFile(record)
	home := filepath.Join(dataHome, "profiles", "work", "home")
	if !strings.Contains(string(got), "HOME="+home) || !strings.Contains(string(got), "XDG_CONFIG_HOME="+filepath.Join(home, ".config")) || !strings.Contains(string(got), "XDG_DATA_HOME="+filepath.Join(home, ".local", "share")) || !strings.Contains(string(got), "XDG_STATE_HOME="+filepath.Join(home, ".local", "state")) {
		t.Fatalf("wrong env: %s", got)
	}
}

func TestRunSelectAfterAgentIsPassthrough(t *testing.T) {
	cfgHome, _, _, project, record := setupRunTest(t)
	writeTestConfig(t, cfgHome, project, "old-profile")
	prompter := &fakePrompter{profile: "new-profile"}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf, Prompter: prompter}.Run([]string{"run", "pi", "--select"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	if prompter.calls != 0 {
		t.Fatalf("prompter calls=%d", prompter.calls)
	}
	got, _ := os.ReadFile(record)
	if !strings.Contains(string(got), "ARGS=--select") {
		t.Fatalf("arg not passed through: %s", got)
	}
}

func TestRunSelectWithoutAgentShowsUsage(t *testing.T) {
	var errBuf bytes.Buffer
	code := App{Err: &errBuf}.Run([]string{"run", "--select"})
	if code != 2 {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(errBuf.String(), "Usage: agentenv run [--select] <agent> [args...]") {
		t.Fatalf("missing usage: %s", errBuf.String())
	}
}

func TestRunForceSelectNonInteractiveDoesNotChangeMapping(t *testing.T) {
	cfgHome, dataHome, _, project, record := setupRunTest(t)
	writeTestConfig(t, cfgHome, project, "old-profile", "new-profile")
	t.Setenv("AGENTENV_NONINTERACTIVE", "1")
	prompter := &fakePrompter{profile: "new-profile"}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf, Prompter: prompter}.Run([]string{"run", "--select", "pi"})
	if code == 0 {
		t.Fatal("expected failure")
	}
	if !strings.Contains(errBuf.String(), "cannot select a profile in non-interactive mode") {
		t.Fatalf("unexpected error: %s", errBuf.String())
	}
	if prompter.calls != 0 {
		t.Fatalf("prompter calls=%d", prompter.calls)
	}
	cfg := loadTestConfig(t, cfgHome)
	key, _ := config.NormalizeProjectPath(project)
	if cfg.Projects[key] != "old-profile" {
		t.Fatalf("mapping changed: %+v", cfg.Projects)
	}
	if _, err := os.Stat(filepath.Join(dataHome, "profiles", "old-profile", "home")); !os.IsNotExist(err) {
		t.Fatalf("profile home was created or stat failed: %v", err)
	}
	if _, err := os.Stat(record); !os.IsNotExist(err) {
		t.Fatalf("agent launched or stat failed: %v", err)
	}
}

func TestRunUnknownLeadingOptionRemainsAgentName(t *testing.T) {
	cfgHome, _, realBin, project, record := setupRunTest(t)
	writeTestConfig(t, cfgHome, project, "old-profile")
	if err := os.WriteFile(filepath.Join(realBin, "--foo"), []byte("#!/bin/sh\nprintf 'AGENT=--foo ARGS=%s' \"$*\" > \""+record+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf}.Run([]string{"run", "--foo"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	got, _ := os.ReadFile(record)
	if !strings.Contains(string(got), "AGENT=--foo") {
		t.Fatalf("did not execute --foo agent: %s", got)
	}
}

func TestRemoveProfileDeletesConfigMappingsAndFolder(t *testing.T) {
	cfgHome := t.TempDir()
	dataHome := t.TempDir()
	project := t.TempDir()
	t.Setenv("AGENTENV_CONFIG_HOME", cfgHome)
	t.Setenv("AGENTENV_HOME", dataHome)
	writeTestConfig(t, cfgHome, project, "old-profile", "keep-profile")
	profileDir := filepath.Join(dataHome, "profiles", "old-profile")
	if err := os.MkdirAll(filepath.Join(profileDir, "home"), 0o700); err != nil {
		t.Fatal(err)
	}
	var out, errBuf bytes.Buffer
	code := App{Out: &out, Err: &errBuf}.Run([]string{"remove", "old-profile"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	assertProfileRemoved(t, cfgHome, profileDir)
}

func TestRemoveWithoutProfileUsesSelector(t *testing.T) {
	cfgHome := t.TempDir()
	dataHome := t.TempDir()
	project := t.TempDir()
	t.Setenv("AGENTENV_CONFIG_HOME", cfgHome)
	t.Setenv("AGENTENV_HOME", dataHome)
	writeTestConfig(t, cfgHome, project, "old-profile", "keep-profile")
	profileDir := filepath.Join(dataHome, "profiles", "old-profile")
	if err := os.MkdirAll(filepath.Join(profileDir, "home"), 0o700); err != nil {
		t.Fatal(err)
	}
	prompter := &fakeRemovePrompter{profile: "old-profile"}
	var out, errBuf bytes.Buffer
	code := App{Out: &out, Err: &errBuf, RemovePrompter: prompter}.Run([]string{"remove"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	if prompter.calls != 1 {
		t.Fatalf("prompter calls=%d", prompter.calls)
	}
	assertProfileRemoved(t, cfgHome, profileDir)
}

func assertProfileRemoved(t *testing.T, cfgHome, profileDir string) {
	t.Helper()
	cfg := loadTestConfig(t, cfgHome)
	if cfg.HasProfile("old-profile") {
		t.Fatalf("profile still in config: %+v", cfg.Profiles)
	}
	if !cfg.HasProfile("keep-profile") {
		t.Fatalf("other profile removed: %+v", cfg.Profiles)
	}
	if len(cfg.Projects) != 0 {
		t.Fatalf("mapping not removed: %+v", cfg.Projects)
	}
	if _, err := os.Stat(profileDir); !os.IsNotExist(err) {
		t.Fatalf("profile folder still exists or stat failed: %v", err)
	}
}

func setupRunTest(t *testing.T) (cfgHome, dataHome, realBin, project, record string) {
	t.Helper()
	cfgHome = t.TempDir()
	dataHome = t.TempDir()
	realBin = t.TempDir()
	project = t.TempDir()
	record = filepath.Join(t.TempDir(), "record")
	t.Setenv("AGENTENV_CONFIG_HOME", cfgHome)
	t.Setenv("AGENTENV_HOME", dataHome)
	t.Setenv("PATH", realBin)
	if err := os.WriteFile(filepath.Join(realBin, "pi"), []byte("#!/bin/sh\nprintf 'HOME=%s\nARGS=%s\n' \"$HOME\" \"$*\" > \""+record+"\"\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	oldWD, _ := os.Getwd()
	t.Cleanup(func() { _ = os.Chdir(oldWD) })
	if err := os.Chdir(project); err != nil {
		t.Fatal(err)
	}
	return cfgHome, dataHome, realBin, project, record
}

func writeTestConfig(t *testing.T, cfgHome, project string, profiles ...string) {
	t.Helper()
	c := config.Empty()
	for _, profile := range profiles {
		if err := c.AddProfile(profile); err != nil {
			t.Fatal(err)
		}
	}
	if err := c.SetProject(project, profiles[0]); err != nil {
		t.Fatal(err)
	}
	if err := config.Save(filepath.Join(cfgHome, "agentenv", "config.toml"), c); err != nil {
		t.Fatal(err)
	}
}

func loadTestConfig(t *testing.T, cfgHome string) config.Config {
	t.Helper()
	cfg, err := config.Load(filepath.Join(cfgHome, "agentenv", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func TestRunCreateWithNoImportCreatesEmptyProfileHome(t *testing.T) {
	_, dataHome, _, _, _ := setupRunTest(t)
	prompter := &fakePrompter{}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf, Prompter: prompter}.Run([]string{"run", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	home := filepath.Join(dataHome, "profiles", "new-profile", "home")
	entries, err := os.ReadDir(home)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 0 {
		t.Fatalf("home not empty: %+v", entries)
	}
}

func TestRunCreateWithOriginalHomeImportCopiesSelectedGroups(t *testing.T) {
	_, dataHome, _, _, _ := setupRunTest(t)
	orig := t.TempDir()
	t.Setenv("HOME", orig)
	writeFile(t, filepath.Join(orig, ".pi/agent/auth.json"), "auth")
	intent := &profileimport.Intent{Source: profileimport.Source{Label: "Original HOME", Path: orig}, GroupIDs: []string{"pi.auth"}}
	prompter := &fakePrompter{importIntent: intent}
	var out, errBuf bytes.Buffer
	code := App{Out: &out, Err: &errBuf, Prompter: prompter}.Run([]string{"run", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	assertTestFile(t, filepath.Join(dataHome, "profiles/new-profile/home/.pi/agent/auth.json"), "auth")
	if !strings.Contains(out.String(), "imported 1 group(s) from Original HOME") {
		t.Fatalf("missing summary: %s", out.String())
	}
}

func TestRunCreateWithExistingProfileImportCopiesSelectedGroups(t *testing.T) {
	cfgHome, dataHome, _, _, _ := setupRunTest(t)
	writeTestConfig(t, cfgHome, t.TempDir(), "source-profile")
	sourceHome := filepath.Join(dataHome, "profiles/source-profile/home")
	writeFile(t, filepath.Join(sourceHome, ".codex/config.toml"), "model=\"x\"")
	intent := &profileimport.Intent{Source: profileimport.Source{Label: "profile: source-profile", Path: sourceHome}, GroupIDs: []string{"codex.config"}}
	prompter := &fakePrompter{importIntent: intent}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf, Prompter: prompter}.Run([]string{"run", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	assertTestFile(t, filepath.Join(dataHome, "profiles/new-profile/home/.codex/config.toml"), "model=\"x\"")
}

func TestRunCreateImportSkipsExistingTargetAndReports(t *testing.T) {
	_, dataHome, _, _, _ := setupRunTest(t)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, ".pi/agent/auth.json"), "source")
	targetAuth := filepath.Join(dataHome, "profiles/new-profile/home/.pi/agent/auth.json")
	writeFile(t, targetAuth, "target")
	intent := &profileimport.Intent{Source: profileimport.Source{Label: "src", Path: src}, GroupIDs: []string{"pi.auth"}}
	var out, errBuf bytes.Buffer
	code := App{Out: &out, Err: &errBuf, Prompter: &fakePrompter{importIntent: intent}}.Run([]string{"run", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	assertTestFile(t, targetAuth, "target")
	if !strings.Contains(out.String(), "skipped existing: .pi/agent/auth.json") {
		t.Fatalf("missing skip summary: %s", out.String())
	}
}

func TestRunCreateImportFailureDoesNotPersistConfigButLeavesPartialFiles(t *testing.T) {
	cfgHome, dataHome, _, project, _ := setupRunTest(t)
	src := t.TempDir()
	writeFile(t, filepath.Join(src, ".pi/agent/auth.json"), "auth")
	mkdirTest(t, filepath.Join(src, ".pi/agent"))
	if err := os.Symlink("elsewhere", filepath.Join(src, ".pi/agent/skills")); err != nil {
		t.Fatal(err)
	}
	intent := &profileimport.Intent{Source: profileimport.Source{Label: "src", Path: src}, GroupIDs: []string{"pi.auth", "pi.skills"}}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf, Prompter: &fakePrompter{importIntent: intent}}.Run([]string{"run", "pi"})
	if code == 0 {
		t.Fatal("expected failure")
	}
	cfg := loadTestConfig(t, cfgHome)
	key, _ := config.NormalizeProjectPath(project)
	if cfg.HasProfile("new-profile") || cfg.Projects[key] != "" {
		t.Fatalf("config persisted after failure: %+v", cfg)
	}
	assertTestFile(t, filepath.Join(dataHome, "profiles/new-profile/home/.pi/agent/auth.json"), "auth")
}

func TestRunExistingProfileDoesNotImport(t *testing.T) {
	cfgHome, dataHome, _, project, _ := setupRunTest(t)
	writeTestConfig(t, cfgHome, project, "old-profile", "new-profile")
	src := t.TempDir()
	writeFile(t, filepath.Join(src, ".pi/agent/auth.json"), "auth")
	intent := &profileimport.Intent{Source: profileimport.Source{Label: "src", Path: src}, GroupIDs: []string{"pi.auth"}}
	prompter := &fakePrompter{profile: "new-profile", importIntent: intent}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf, Prompter: prompter}.Run([]string{"run", "--select", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	if _, err := os.Stat(filepath.Join(dataHome, "profiles/new-profile/home/.pi/agent/auth.json")); !os.IsNotExist(err) {
		t.Fatalf("import occurred or stat failed: %v", err)
	}
}

func TestRunPrompterReceivesOriginalHomeAndProfileSources(t *testing.T) {
	cfgHome, dataHome, _, _, _ := setupRunTest(t)
	orig := t.TempDir()
	t.Setenv("HOME", orig)
	writeTestConfig(t, cfgHome, t.TempDir(), "source-profile")
	mkdirTest(t, filepath.Join(dataHome, "profiles/source-profile/home"))
	prompter := &fakePrompter{}
	var errBuf bytes.Buffer
	code := App{Err: &errBuf, Prompter: prompter}.Run([]string{"run", "pi"})
	if code != 0 {
		t.Fatalf("code=%d err=%s", code, errBuf.String())
	}
	if len(prompter.sources) != 2 || prompter.sources[0].Kind != profileimport.SourceKindHome || prompter.sources[1].ID != "profile:source-profile" {
		t.Fatalf("sources=%+v", prompter.sources)
	}
}

func mkdirTest(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o700); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	mkdirTest(t, filepath.Dir(path))
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func assertTestFile(t *testing.T, path, content string) {
	t.Helper()
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("%s=%q want %q", path, got, content)
	}
}
