package e2e

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/flobilosaurus/agent-env/internal/config"
)

func TestRunMappedProjectSetsHomeAndArgs(t *testing.T) {
	bin := buildAgentenv(t)
	cfg := t.TempDir()
	data := t.TempDir()
	project := t.TempDir()
	realBin := t.TempDir()
	rec := filepath.Join(t.TempDir(), "record")
	writeConfig(t, cfg, project, "customer-a")
	fakeAgent(t, realBin, "pi", "#!/bin/sh\nprintf 'HOME=%s\nPWD=%s\nARGS=%s\n' \"$HOME\" \"$PWD\" \"$*\" > \""+rec+"\"\nexit 0\n")
	cmd := exec.Command(bin, "run", "pi", "--foo", "bar baz")
	cmd.Dir = project
	cmd.Env = baseEnv(cfg, data, realBin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	if !strings.Contains(string(out), "│ Profile  customer-a") || !strings.Contains(string(out), "│ Agent    pi") {
		t.Fatalf("missing banner: %s", out)
	}
	got, _ := os.ReadFile(rec)
	s := string(got)
	wantHome := filepath.Join(data, "profiles", "customer-a", "home")
	if !strings.Contains(s, "HOME="+wantHome) || !strings.Contains(s, "ARGS=--foo bar baz") {
		t.Fatalf("record wrong:\n%s", s)
	}
}

func TestRunSetsExplicitEnvironment(t *testing.T) {
	bin := buildAgentenv(t)
	cfg := t.TempDir()
	data := t.TempDir()
	project := t.TempDir()
	realBin := t.TempDir()
	rec := filepath.Join(t.TempDir(), "record")
	writeConfig(t, cfg, project, "customer-a")
	fakeAgent(t, realBin, "test-agent", "#!/bin/sh\nprintf 'DOCKER_CONFIG=%s\nARGS=%s\n' \"$DOCKER_CONFIG\" \"$*\" > \""+rec+"\"\n")
	dockerConfig := filepath.Join(t.TempDir(), ".docker")
	cmd := exec.Command(bin, "run", "--env", "DOCKER_CONFIG="+dockerConfig, "test-agent", "run", "pi")
	cmd.Dir = project
	cmd.Env = baseEnv(cfg, data, realBin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(rec)
	if !strings.Contains(string(got), "DOCKER_CONFIG="+dockerConfig) || !strings.Contains(string(got), "ARGS=run pi") {
		t.Fatalf("record wrong: %s", got)
	}
}

func TestRunSkipsWrapperBinAndPropagatesExit(t *testing.T) {
	bin := buildAgentenv(t)
	cfg := t.TempDir()
	data := t.TempDir()
	project := t.TempDir()
	wrap := filepath.Join(data, "bin")
	realBin := t.TempDir()
	os.MkdirAll(wrap, 0755)
	writeConfig(t, cfg, project, "p")
	fakeAgent(t, wrap, "pi", "#!/bin/sh\nexit 88\n")
	fakeAgent(t, realBin, "pi", "#!/bin/sh\nexit 7\n")
	cmd := exec.Command(bin, "run", "pi")
	cmd.Dir = project
	cmd.Env = baseEnv(cfg, data, wrap+string(os.PathListSeparator)+realBin)
	err := cmd.Run()
	if err == nil {
		t.Fatal("expected exit")
	}
	if ee := err.(*exec.ExitError); ee.ExitCode() != 7 {
		t.Fatalf("exit=%d", ee.ExitCode())
	}
}

func TestRunUnmappedNonInteractiveFails(t *testing.T) {
	bin := buildAgentenv(t)
	cfg := t.TempDir()
	data := t.TempDir()
	project := t.TempDir()
	realBin := t.TempDir()
	fakeAgent(t, realBin, "pi", "#!/bin/sh\nexit 0\n")
	cmd := exec.Command(bin, "run", "pi")
	cmd.Dir = project
	cmd.Env = baseEnv(cfg, data, realBin)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected fail")
	}
	if !strings.Contains(string(out), "no profile mapping") {
		t.Fatalf("unexpected: %s", out)
	}
	if _, err := os.Stat(filepath.Join(data, "profiles")); !os.IsNotExist(err) {
		t.Fatalf("profile side effect exists or stat failed: %v", err)
	}
}

func TestRunForceSelectNonInteractiveFailsWithoutMutatingMappingOrLaunching(t *testing.T) {
	bin := buildAgentenv(t)
	cfg := t.TempDir()
	data := t.TempDir()
	project := t.TempDir()
	realBin := t.TempDir()
	rec := filepath.Join(t.TempDir(), "record")
	writeConfig(t, cfg, project, "customer-a")
	fakeAgent(t, realBin, "pi", "#!/bin/sh\nprintf launched > \""+rec+"\"\n")
	cmd := exec.Command(bin, "run", "--select", "pi")
	cmd.Dir = project
	cmd.Env = baseEnv(cfg, data, realBin)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected fail")
	}
	if !strings.Contains(string(out), "cannot select a profile in non-interactive mode") {
		t.Fatalf("unexpected: %s", out)
	}
	loaded, err := config.Load(filepath.Join(cfg, "agentenv", "config.toml"))
	if err != nil {
		t.Fatal(err)
	}
	key, _ := config.NormalizeProjectPath(project)
	if loaded.Projects[key] != "customer-a" {
		t.Fatalf("mapping changed: %+v", loaded.Projects)
	}
	if _, err := os.Stat(rec); !os.IsNotExist(err) {
		t.Fatalf("agent launched or stat failed: %v", err)
	}
}

func TestRunSelectAfterAgentIsPassthroughArgument(t *testing.T) {
	bin := buildAgentenv(t)
	cfg := t.TempDir()
	data := t.TempDir()
	project := t.TempDir()
	realBin := t.TempDir()
	rec := filepath.Join(t.TempDir(), "record")
	writeConfig(t, cfg, project, "customer-a")
	fakeAgent(t, realBin, "pi", "#!/bin/sh\nprintf 'ARGS=%s' \"$*\" > \""+rec+"\"\n")
	cmd := exec.Command(bin, "run", "pi", "--select")
	cmd.Dir = project
	cmd.Env = baseEnv(cfg, data, realBin)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run failed: %v\n%s", err, out)
	}
	got, _ := os.ReadFile(rec)
	if !strings.Contains(string(got), "ARGS=--select") {
		t.Fatalf("arg not passed through: %s", got)
	}
}

func TestRunSelectWithoutAgentShowsUsageAndExits2(t *testing.T) {
	bin := buildAgentenv(t)
	cfg := t.TempDir()
	data := t.TempDir()
	realBin := t.TempDir()
	cmd := exec.Command(bin, "run", "--select")
	cmd.Env = baseEnv(cfg, data, realBin)
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected fail")
	}
	if ee := err.(*exec.ExitError); ee.ExitCode() != 2 {
		t.Fatalf("exit=%d output=%s", ee.ExitCode(), out)
	}
	if !strings.Contains(string(out), "Usage: agentenv run [--select] [--env KEY=VALUE]... <agent> [args...]") {
		t.Fatalf("missing usage: %s", out)
	}
}
