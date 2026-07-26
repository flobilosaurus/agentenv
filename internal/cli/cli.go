package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/x/term"

	"github.com/flobilosaurus/agent-env/internal/config"
	"github.com/flobilosaurus/agent-env/internal/doctor"
	"github.com/flobilosaurus/agent-env/internal/paths"
	"github.com/flobilosaurus/agent-env/internal/pathsetup"
	"github.com/flobilosaurus/agent-env/internal/profileimport"
	"github.com/flobilosaurus/agent-env/internal/runner"
	"github.com/flobilosaurus/agent-env/internal/tui"
	"github.com/flobilosaurus/agent-env/internal/wrapper"
)

var Version = "dev"

const launchBannerDelay = 1 * time.Second

type App struct {
	In             io.Reader
	Out, Err       io.Writer
	Prompter       tui.ProfilePrompter
	RemovePrompter tui.ProfileRemovePrompter
	UnwrapPrompter tui.WrapperUnwrapPrompter
}

func (a App) Run(args []string) int {
	if a.In == nil {
		a.In = os.Stdin
	}
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.Err == nil {
		a.Err = os.Stderr
	}
	if len(args) == 0 {
		fmt.Fprint(a.Err, Usage())
		return 2
	}
	switch args[0] {
	case "help", "--help", "-h":
		fmt.Fprint(a.Out, Usage())
		return 0
	case "version", "--version", "-v":
		fmt.Fprintln(a.Out, "agentenv", Version)
		return 0
	case "run":
		return a.run(args[1:])
	case "wrap":
		return a.wrap(args[1:])
	case "unwrap":
		return a.unwrap(args[1:])
	case "remove":
		return a.remove(args[1:])
	case "doctor":
		return a.doctor(args[1:])
	default:
		fmt.Fprintf(a.Err, "agentenv: unknown command %q\n\n%s", args[0], Usage())
		return 2
	}
}

func Usage() string {
	return `Usage: agentenv <command> [args]

Commands:
  run [--select] [--env KEY=VALUE]... <agent> [args...]
                                     Run an agent with project profile HOME isolation
  wrap <agent>                       Install a wrapper into the agentenv bin directory
  unwrap                             Select and delete an agentenv wrapper binary
  remove [profile]                   Remove a profile, its mappings, and its folder
  doctor [agent]                     Check config, mappings, profile homes, and PATH
  version                            Print version
  help                               Print help
`
}

func (a App) commandUsage(cmd string) string {
	switch cmd {
	case "run":
		return "Usage: agentenv run [--select] [--env KEY=VALUE]... <agent> [args...]\n"
	case "wrap":
		return "Usage: agentenv wrap <agent>\n"
	case "unwrap":
		return "Usage: agentenv unwrap\n"
	case "remove":
		return "Usage: agentenv remove [profile]\n"
	case "doctor":
		return "Usage: agentenv doctor [agent]\n"
	}
	return Usage()
}

func parseRunArgs(args []string) (forceSelect bool, runEnv map[string]string, agent string, pass []string, err error) {
	runEnv = make(map[string]string)
	for i := 0; i < len(args); i++ {
		switch {
		case args[i] == "--select":
			forceSelect = true
		case args[i] == "--env":
			i++
			if i >= len(args) {
				return false, nil, "", nil, fmt.Errorf("--env requires KEY=VALUE")
			}
			key, value, err := parseEnvAssignment(args[i])
			if err != nil {
				return false, nil, "", nil, err
			}
			runEnv[key] = value
		case strings.HasPrefix(args[i], "--env="):
			key, value, err := parseEnvAssignment(strings.TrimPrefix(args[i], "--env="))
			if err != nil {
				return false, nil, "", nil, err
			}
			runEnv[key] = value
		default:
			return forceSelect, runEnv, args[i], args[i+1:], nil
		}
	}
	return false, nil, "", nil, fmt.Errorf("agent is required")
}

func parseEnvAssignment(assignment string) (string, string, error) {
	key, value, ok := strings.Cut(assignment, "=")
	if !ok || !validEnvName(key) {
		return "", "", fmt.Errorf("invalid environment assignment %q; expected KEY=VALUE", assignment)
	}
	if key == "HOME" {
		return "", "", fmt.Errorf("cannot override HOME; agentenv uses it for profile isolation")
	}
	return key, value, nil
}

func validEnvName(name string) bool {
	for i, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || r == '_' || (i > 0 && r >= '0' && r <= '9') {
			continue
		}
		return false
	}
	return name != ""
}

func (a App) run(args []string) int {
	forceSelect, runEnv, agent, pass, err := parseRunArgs(args)
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		fmt.Fprint(a.Err, a.commandUsage("run"))
		return 2
	}
	p, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	cfgPath := p.ConfigFile()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv: config:", err)
		return 1
	}
	cwd, _ := os.Getwd()
	project, err := config.NormalizeProjectPath(cwd)
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	profile := cfg.Projects[project]
	if profile == "" || forceSelect {
		if os.Getenv("AGENTENV_NONINTERACTIVE") == "1" {
			if forceSelect {
				fmt.Fprintf(a.Err, "agentenv: cannot select a profile in non-interactive mode for %s\n", project)
			} else {
				fmt.Fprintf(a.Err, "agentenv: no profile mapping for %s\n", project)
			}
			return 1
		}
		chosen, err := a.chooseAndSaveProfile(p, cfgPath, &cfg, project, agent)
		if err != nil {
			fmt.Fprintln(a.Err, "agentenv:", err)
			return 1
		}
		profile = chosen
	}
	if !cfg.HasProfile(profile) {
		fmt.Fprintf(a.Err, "agentenv: mapped profile %q does not exist\n", profile)
		return 1
	}
	home, err := paths.EnsureProfileHome(p, profile)
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv: profile home:", err)
		return 1
	}
	real, err := runner.LookupAgent(agent, p.BinDir())
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 127
	}
	termWidth := terminalWidth(a.Out)
	fmt.Fprintln(a.Out, tui.BannerWithWidth(profile, agent, termWidth))
	if termWidth > 0 {
		time.Sleep(launchBannerDelay)
	}
	extraEnv := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
	}
	if agent == "claude" {
		extraEnv["CLAUDE_CONFIG_DIR"] = filepath.Join(home, ".claude")
	}
	for key, value := range runEnv {
		extraEnv[key] = value
	}
	return runner.RunAgentWithEnv(real, pass, home, extraEnv, runner.IO{Stdin: a.In, Stdout: a.Out, Stderr: a.Err})
}

func terminalWidth(w io.Writer) int {
	file, ok := w.(*os.File)
	if !ok {
		return 0
	}
	width, _, err := term.GetSize(file.Fd())
	if err != nil {
		return 0
	}
	return width
}

func (a App) chooseAndSaveProfile(p paths.Paths, cfgPath string, cfg *config.Config, project, agent string) (string, error) {
	prompter := a.Prompter
	if prompter == nil {
		prompter = tui.BubblePrompter{}
	}
	catalog := profileimport.Catalog()
	sources := profileimport.ProfileSources(p, cfg.Profiles, os.Getenv("HOME"))
	choice, err := prompter.ChooseProfile(agent, cfg.Profiles, sources, catalog)
	if err != nil {
		return "", err
	}
	chosen := choice.Profile
	next := cloneConfig(*cfg)
	if choice.Create {
		if err := next.AddProfile(chosen); err != nil {
			return "", err
		}
		home, err := paths.EnsureProfileHome(p, chosen)
		if err != nil {
			return "", fmt.Errorf("profile home: %w", err)
		}
		if choice.Import != nil && len(choice.Import.GroupIDs) > 0 {
			result, err := profileimport.ImportSelection(home, *choice.Import, catalog)
			if err != nil {
				return "", err
			}
			a.printImportSummary(chosen, choice.Import.Source, result)
		}
	} else if !next.HasProfile(chosen) {
		return "", fmt.Errorf("profile %q does not exist", chosen)
	}
	if err := next.SetProject(project, chosen); err != nil {
		return "", err
	}
	if err := config.Save(cfgPath, next); err != nil {
		return "", fmt.Errorf("save config: %w", err)
	}
	*cfg = next
	return chosen, nil
}

func cloneConfig(c config.Config) config.Config {
	clone := config.Config{Profiles: append([]config.Profile(nil), c.Profiles...), Projects: map[string]string{}}
	for project, profile := range c.Projects {
		clone.Projects[project] = profile
	}
	return clone
}

func (a App) printImportSummary(profile string, source profileimport.Source, result profileimport.Result) {
	if len(result.Copied) == 0 && len(result.Skipped) == 0 {
		return
	}
	fmt.Fprintf(a.Out, "imported %d group(s) from %s into profile %q\n", len(result.Copied), source.Label, profile)
	for _, skipped := range result.Skipped {
		fmt.Fprintf(a.Out, "skipped existing: %s\n", skipped.Path)
	}
}

func (a App) wrap(args []string) int {
	if len(args) != 1 {
		fmt.Fprint(a.Err, a.commandUsage("wrap"))
		return 2
	}
	p, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	exe, err := os.Executable()
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	exe, _ = filepath.Abs(exe)
	target, err := wrapper.Install(p.BinDir(), exe, args[0])
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	fmt.Fprintf(a.Out, "installed wrapper: %s\n", target)
	pathResult, err := pathsetup.EnsureWrapperBinFirst(p.BinDir())
	if err != nil {
		fmt.Fprintf(a.Err, "agentenv: could not update PATH: %v\n", err)
		fmt.Fprintf(a.Err, "hint: add %s before real agent binaries on PATH, then restart your shell.\n", p.BinDir())
		fmt.Fprintf(a.Err, "hint: until then, use `agentenv run %s` directly.\n", args[0])
		return 0
	}
	if pathResult.Changed {
		fmt.Fprintf(a.Out, "updated PATH setup: %s\nRestart your shell or source that file before running %s directly.\n", pathResult.ProfilePath, args[0])
	} else {
		fmt.Fprintf(a.Out, "PATH setup already up to date: %s\n", pathResult.ProfilePath)
	}
	return 0
}

func (a App) unwrap(args []string) int {
	if len(args) != 0 {
		fmt.Fprint(a.Err, a.commandUsage("unwrap"))
		return 2
	}
	p, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	agents, err := wrapper.List(p.BinDir())
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv: list wrappers:", err)
		return 1
	}
	if len(agents) == 0 {
		fmt.Fprintln(a.Err, "agentenv: no wrappers to unwrap")
		return 1
	}
	if os.Getenv("AGENTENV_NONINTERACTIVE") == "1" {
		fmt.Fprintln(a.Err, "agentenv: cannot select a wrapper in non-interactive mode")
		return 1
	}
	prompter := a.UnwrapPrompter
	if prompter == nil {
		prompter = tui.BubblePrompter{}
	}
	agent, err := prompter.ChooseWrapperToUnwrap(agents)
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	target, err := wrapper.Uninstall(p.BinDir(), agent)
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	fmt.Fprintf(a.Out, "removed wrapper: %s\n", target)
	return 0
}

func (a App) remove(args []string) int {
	if len(args) > 1 {
		fmt.Fprint(a.Err, a.commandUsage("remove"))
		return 2
	}
	p, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	cfgPath := p.ConfigFile()
	cfg, err := config.Load(cfgPath)
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv: config:", err)
		return 1
	}
	profile := ""
	if len(args) == 1 {
		profile = args[0]
	} else {
		if len(cfg.Profiles) == 0 {
			fmt.Fprintln(a.Err, "agentenv: no profiles to remove")
			return 1
		}
		if os.Getenv("AGENTENV_NONINTERACTIVE") == "1" {
			fmt.Fprintln(a.Err, "agentenv: cannot select a profile in non-interactive mode")
			return 1
		}
		prompter := a.RemovePrompter
		if prompter == nil {
			prompter = tui.BubblePrompter{}
		}
		profile, err = prompter.ChooseProfileToRemove(cfg.Profiles)
		if err != nil {
			fmt.Fprintln(a.Err, "agentenv:", err)
			return 1
		}
	}
	if err := cfg.RemoveProfile(profile); err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	profileDir := p.ProfileDir(profile)
	if err := removeAll(profileDir); err != nil {
		fmt.Fprintln(a.Err, "agentenv: remove profile folder:", err)
		return 1
	}
	if err := config.Save(cfgPath, cfg); err != nil {
		fmt.Fprintln(a.Err, "agentenv: save config:", err)
		return 1
	}
	fmt.Fprintf(a.Out, "removed profile %q and folder: %s\n", profile, profileDir)
	return 0
}

// removeAll is like os.RemoveAll but first makes directories writable so that
// read-only trees (e.g. Go module cache) can be deleted without permission errors.
func removeAll(path string) error {
	_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
		if err == nil && d.IsDir() {
			_ = os.Chmod(p, 0o755)
		}
		return nil
	})
	return os.RemoveAll(path)
}

func (a App) doctor(args []string) int {
	if len(args) > 1 {
		fmt.Fprint(a.Err, a.commandUsage("doctor"))
		return 2
	}
	p, err := paths.Resolve()
	if err != nil {
		fmt.Fprintln(a.Err, "agentenv:", err)
		return 1
	}
	cwd, _ := os.Getwd()
	agent := ""
	if len(args) == 1 {
		agent = args[0]
	}
	r := doctor.Run(cwd, agent, p)
	fmt.Fprint(a.Out, doctor.Format(r))
	return r.ExitCode()
}
