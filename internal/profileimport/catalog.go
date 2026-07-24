package profileimport

type Kind string

const (
	KindFile Kind = "file"
	KindDir  Kind = "dir"
)

type Group struct {
	ID        string
	Agent     string
	Label     string
	SourceRel string
	TargetRel string
	Kind      Kind
}

func Catalog() []Group {
	return []Group{
		{ID: "pi.agents-md", Agent: "pi", Label: "AGENTS.md", SourceRel: ".pi/agent/AGENTS.md", TargetRel: ".pi/agent/AGENTS.md", Kind: KindFile},
		{ID: "pi.auth", Agent: "pi", Label: "auth", SourceRel: ".pi/agent/auth.json", TargetRel: ".pi/agent/auth.json", Kind: KindFile},
		{ID: "pi.settings", Agent: "pi", Label: "settings", SourceRel: ".pi/agent/settings.json", TargetRel: ".pi/agent/settings.json", Kind: KindFile},
		{ID: "pi.models", Agent: "pi", Label: "models", SourceRel: ".pi/agent/models.json", TargetRel: ".pi/agent/models.json", Kind: KindFile},
		{ID: "pi.trust", Agent: "pi", Label: "trust", SourceRel: ".pi/agent/trust.json", TargetRel: ".pi/agent/trust.json", Kind: KindFile},
		{ID: "pi.keybindings", Agent: "pi", Label: "keybindings", SourceRel: ".pi/agent/keybindings.json", TargetRel: ".pi/agent/keybindings.json", Kind: KindFile},
		{ID: "pi.extensions", Agent: "pi", Label: "extensions", SourceRel: ".pi/agent/extensions", TargetRel: ".pi/agent/extensions", Kind: KindDir},
		{ID: "pi.skills", Agent: "pi", Label: "skills", SourceRel: ".pi/agent/skills", TargetRel: ".pi/agent/skills", Kind: KindDir},
		{ID: "pi.prompts", Agent: "pi", Label: "prompts", SourceRel: ".pi/agent/prompts", TargetRel: ".pi/agent/prompts", Kind: KindDir},
		{ID: "pi.themes", Agent: "pi", Label: "themes", SourceRel: ".pi/agent/themes", TargetRel: ".pi/agent/themes", Kind: KindDir},
		{ID: "pi.git", Agent: "pi", Label: "git", SourceRel: ".pi/agent/git", TargetRel: ".pi/agent/git", Kind: KindDir},
		{ID: "pi.npm", Agent: "pi", Label: "npm", SourceRel: ".pi/agent/npm", TargetRel: ".pi/agent/npm", Kind: KindDir},
		{ID: "claude.state", Agent: "claude", Label: "state", SourceRel: ".claude.json", TargetRel: ".claude/.claude.json", Kind: KindFile},
		{ID: "claude.config-state", Agent: "claude", Label: "config state", SourceRel: ".claude/.claude.json", TargetRel: ".claude/.claude.json", Kind: KindFile},
		{ID: "claude.credentials", Agent: "claude", Label: "credentials", SourceRel: ".claude/.credentials.json", TargetRel: ".claude/.credentials.json", Kind: KindFile},
		{ID: "claude.settings", Agent: "claude", Label: "settings", SourceRel: ".claude/settings.json", TargetRel: ".claude/settings.json", Kind: KindFile},
		{ID: "claude.claude-md", Agent: "claude", Label: "CLAUDE.md", SourceRel: ".claude/CLAUDE.md", TargetRel: ".claude/CLAUDE.md", Kind: KindFile},
		{ID: "claude.agents", Agent: "claude", Label: "agents", SourceRel: ".claude/agents", TargetRel: ".claude/agents", Kind: KindDir},
		{ID: "claude.skills", Agent: "claude", Label: "skills", SourceRel: ".claude/skills", TargetRel: ".claude/skills", Kind: KindDir},
		{ID: "claude.commands", Agent: "claude", Label: "commands", SourceRel: ".claude/commands", TargetRel: ".claude/commands", Kind: KindDir},
		{ID: "claude.hooks", Agent: "claude", Label: "hooks", SourceRel: ".claude/hooks", TargetRel: ".claude/hooks", Kind: KindDir},
		{ID: "claude.plugins", Agent: "claude", Label: "plugins", SourceRel: ".claude/plugins", TargetRel: ".claude/plugins", Kind: KindDir},
		{ID: "claude.themes", Agent: "claude", Label: "themes", SourceRel: ".claude/themes", TargetRel: ".claude/themes", Kind: KindDir},
		{ID: "opencode.config", Agent: "opencode", Label: "config", SourceRel: ".config/opencode/opencode.json", TargetRel: ".config/opencode/opencode.json", Kind: KindFile},
		{ID: "opencode.tui", Agent: "opencode", Label: "tui", SourceRel: ".config/opencode/tui.json", TargetRel: ".config/opencode/tui.json", Kind: KindFile},
		{ID: "opencode.agents-md", Agent: "opencode", Label: "AGENTS.md", SourceRel: ".config/opencode/AGENTS.md", TargetRel: ".config/opencode/AGENTS.md", Kind: KindFile},
		{ID: "opencode.agents", Agent: "opencode", Label: "agents", SourceRel: ".config/opencode/agents", TargetRel: ".config/opencode/agents", Kind: KindDir},
		{ID: "opencode.commands", Agent: "opencode", Label: "commands", SourceRel: ".config/opencode/commands", TargetRel: ".config/opencode/commands", Kind: KindDir},
		{ID: "opencode.plugins", Agent: "opencode", Label: "plugins", SourceRel: ".config/opencode/plugins", TargetRel: ".config/opencode/plugins", Kind: KindDir},
		{ID: "opencode.skills", Agent: "opencode", Label: "skills", SourceRel: ".config/opencode/skills", TargetRel: ".config/opencode/skills", Kind: KindDir},
		{ID: "opencode.themes", Agent: "opencode", Label: "themes", SourceRel: ".config/opencode/themes", TargetRel: ".config/opencode/themes", Kind: KindDir},
		{ID: "opencode.tools", Agent: "opencode", Label: "tools", SourceRel: ".config/opencode/tools", TargetRel: ".config/opencode/tools", Kind: KindDir},
		{ID: "opencode.modes", Agent: "opencode", Label: "modes", SourceRel: ".config/opencode/modes", TargetRel: ".config/opencode/modes", Kind: KindDir},
		{ID: "opencode.auth", Agent: "opencode", Label: "auth", SourceRel: ".local/share/opencode/auth.json", TargetRel: ".local/share/opencode/auth.json", Kind: KindFile},
		{ID: "opencode.state", Agent: "opencode", Label: "state", SourceRel: ".local/state/opencode", TargetRel: ".local/state/opencode", Kind: KindDir},
		{ID: "codex.config", Agent: "codex", Label: "config", SourceRel: ".codex/config.toml", TargetRel: ".codex/config.toml", Kind: KindFile},
		{ID: "codex.auth", Agent: "codex", Label: "auth", SourceRel: ".codex/auth.json", TargetRel: ".codex/auth.json", Kind: KindFile},
		{ID: "codex.history", Agent: "codex", Label: "history", SourceRel: ".codex/history.jsonl", TargetRel: ".codex/history.jsonl", Kind: KindFile},
		{ID: "codex.skills", Agent: "codex", Label: "skills", SourceRel: ".codex/skills", TargetRel: ".codex/skills", Kind: KindDir},
	}
}
