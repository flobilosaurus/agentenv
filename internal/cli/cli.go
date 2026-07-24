package cli

import (
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/flobilosaurus/agent-env/internal/config"
	"github.com/flobilosaurus/agent-env/internal/doctor"
	"github.com/flobilosaurus/agent-env/internal/paths"
	"github.com/flobilosaurus/agent-env/internal/pathsetup"
	"github.com/flobilosaurus/agent-env/internal/profileimport"
	"github.com/flobilosaurus/agent-env/internal/runner"
	"github.com/flobilosaurus/agent-env/internal/tui"
	"github.com/flobilosaurus/agent-env/internal/wrapper"
)

const Version = "0.1.0"

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
  run [--select] <agent> [args...]   Run an agent with project profile HOME isolation
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
		return "Usage: agentenv run [--select] <agent> [args...]\n"
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

func parseRunArgs(args []string) (forceSelect bool, agent string, pass []string, ok bool) {
	if len(args) == 0 {
		return false, "", nil, false
	}
	if args[0] == "--select" {
		if len(args) == 1 {
			return true, "", nil, false
		}
		return true, args[1], args[2:], true
	}
	return false, args[0], args[1:], true
}

func (a App) run(args []string) int {
	forceSelect, agent, pass, ok := parseRunArgs(args)
	if !ok {
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
	fmt.Fprintln(a.Out, tui.Banner(profile, agent))
	extraEnv := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
	}
	if agent == "claude" {
		extraEnv["CLAUDE_CONFIG_DIR"] = filepath.Join(home, ".claude")
	}
	return runner.RunAgentWithEnv(real, pass, home, extraEnv, runner.IO{Stdin: a.In, Stdout: a.Out, Stderr: a.Err})
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
	if err := os.RemoveAll(profileDir); err != nil {
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
