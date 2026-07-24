package tui

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/flobilosaurus/agent-env/internal/config"
	"github.com/flobilosaurus/agent-env/internal/profileimport"
)

func TestBanner(t *testing.T) {
	want := "┌─ agentenv ───────────────────────────────────┐\n" +
		"│ work • pi                                    │\n" +
		"└──────────────────────────────────────────────┘"
	if got := Banner("work", "pi"); got != want {
		t.Fatalf("banner mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestProfileSelectionView(t *testing.T) {
	m := newModel("pi", []config.Profile{{Name: "customer-a"}, {Name: "customer-b"}, {Name: "personal"}})
	want := "╭─ agentenv ───────────────────────────────────────────────╮\n" +
		"│ customer-a • pi                                          │\n" +
		"├──────────────────────────────────────────────────────────┤\n" +
		"│                                                          │\n" +
		"│  Select a Profile                                        │\n" +
		"│  Choose an isolated HOME for this project                │\n" +
		"│                                                          │\n" +
		"│  ▸ customer-a                                            │\n" +
		"│    customer-b                                            │\n" +
		"│    personal                                              │\n" +
		"│                                                          │\n" +
		"│    ＋ Create new profile                                 │\n" +
		"│                                                          │\n" +
		"│  ↑/↓/j/k move • enter select • esc/ctrl+c cancel         │\n" +
		"╰──────────────────────────────────────────────────────────╯"
	if got := m.View(); got != want {
		t.Fatalf("selection view mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestProfileCreateView(t *testing.T) {
	m := newModel("pi", []config.Profile{{Name: "customer-a"}})
	m.mode = modeCreate
	want := "╭─ agentenv ───────────────────────────────────────────────╮\n" +
		"│  • pi                                                    │\n" +
		"├──────────────────────────────────────────────────────────┤\n" +
		"│                                                          │\n" +
		"│  Create a Profile                                        │\n" +
		"│  Allowed: lowercase, numbers, dot, dash, underscore      │\n" +
		"│                                                          │\n" +
		"│  > profile-name                                          │\n" +
		"│                                                          │\n" +
		"│  enter continue • esc/ctrl+c cancel                      │\n" +
		"╰──────────────────────────────────────────────────────────╯"
	if got := m.View(); got != want {
		t.Fatalf("create view mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestProfileCreateViewShowsTypedProfile(t *testing.T) {
	m := newModel("pi", []config.Profile{{Name: "customer-a"}})
	m.mode = modeCreate
	m.input.SetValue("new-profile")
	if got := m.View(); !strings.Contains(got, "│ new-profile • pi") {
		t.Fatalf("create view should show typed profile in header\ngot:\n%s", got)
	}
}

func TestProfileSelectionCreateRowShowsBlankProfile(t *testing.T) {
	m := newModel("pi", []config.Profile{{Name: "customer-a"}})
	m.cursor = len(m.profiles)
	if got := m.View(); !strings.Contains(got, "│  • pi") {
		t.Fatalf("selection create row should show blank profile in header\ngot:\n%s", got)
	}
}

func TestProfileRemoveView(t *testing.T) {
	m := newRemoveModel([]config.Profile{{Name: "customer-a"}, {Name: "customer-b"}})
	want := "╭─ agentenv ───────────────────────────────────────────────╮\n" +
		"│                                                          │\n" +
		"│  Remove a Profile                                        │\n" +
		"│  Select a profile to delete with its folder              │\n" +
		"│                                                          │\n" +
		"│  ▸ customer-a                                            │\n" +
		"│    customer-b                                            │\n" +
		"│                                                          │\n" +
		"│  ↑/↓/j/k move • enter remove • esc/ctrl+c cancel         │\n" +
		"╰──────────────────────────────────────────────────────────╯"
	if got := m.View(); got != want {
		t.Fatalf("remove view mismatch\nwant:\n%s\ngot:\n%s", want, got)
	}
}

func TestImportSourceSelectionView(t *testing.T) {
	m := newModel("pi", nil, []profileimport.Source{{Label: "Original HOME", Path: "/Users/me"}, {Label: "profile: work", Path: "/data/profiles/work/home"}}, profileimport.Catalog())
	m.selected = "new-profile"
	m.created = true
	m.mode = modeImportSource
	got := m.View()
	for _, want := range []string{"Import from", "▸ No import", "Original HOME  /Users/me", "profile: work  /data/profiles/work/home", "enter select"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
}

func TestImportGroupMultiSelectRendersOnlyAvailableGroups(t *testing.T) {
	home := t.TempDir()
	writeTUITestFile(t, filepath.Join(home, ".pi/agent/auth.json"), "auth")
	writeTUITestFile(t, filepath.Join(home, ".codex/config.toml"), "cfg")
	m := newModel("pi", nil, []profileimport.Source{{Label: "Original HOME", Path: home}}, profileimport.Catalog())
	m.selected = "new-profile"
	m.created = true
	m.mode = modeImportSource
	m.sourceCursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	fm := updated.(model)
	got := fm.View()
	if !strings.Contains(got, "[x] pi") || !strings.Contains(got, "auth") || !strings.Contains(got, "[x] codex") || !strings.Contains(got, "config") {
		t.Fatalf("missing available groups in\n%s", got)
	}
	if strings.Contains(got, "skills") {
		t.Fatalf("rendered unavailable group in\n%s", got)
	}
}

func TestImportGroupSpaceTogglesSelectedGroup(t *testing.T) {
	m := groupModeModel([]profileimport.Group{{ID: "pi.auth", Agent: "pi", Label: "auth", SourceRel: ".pi/agent/auth.json"}})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeySpace})
	fm := updated.(model)
	if fm.selectedGroups["pi.auth"] {
		t.Fatalf("group still selected")
	}
}

func TestImportGroupsSelectedByDefault(t *testing.T) {
	home := t.TempDir()
	writeTUITestFile(t, filepath.Join(home, ".pi/agent/auth.json"), "auth")
	m := newModel("pi", nil, []profileimport.Source{{Label: "Original HOME", Path: home}}, profileimport.Catalog())
	m.selected = "new-profile"
	m.mode = modeImportSource
	m.sourceCursor = 1
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	fm := updated.(model)
	if !fm.selectedGroups["pi.auth"] {
		t.Fatalf("default not selected: %+v", fm.selectedGroups)
	}
}

func TestImportGroupAllAndNoneKeys(t *testing.T) {
	m := groupModeModel([]profileimport.Group{{ID: "a"}, {ID: "b"}})
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	fm := updated.(model)
	if fm.selectedGroups["a"] || fm.selectedGroups["b"] {
		t.Fatalf("none failed: %+v", fm.selectedGroups)
	}
	updated, _ = fm.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	fm = updated.(model)
	if !fm.selectedGroups["a"] || !fm.selectedGroups["b"] {
		t.Fatalf("all failed: %+v", fm.selectedGroups)
	}
}

func TestConfirmingNoImportReturnsNilIntent(t *testing.T) {
	m := newModel("pi", nil)
	m.selected = "new-profile"
	m.created = true
	m.mode = modeImportSource
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	fm := updated.(model)
	if fm.importIntent != nil || !fm.created || fm.selected != "new-profile" {
		t.Fatalf("choice=%+v", fm)
	}
}

func TestImportModesEscapeCancelWizard(t *testing.T) {
	for _, mode := range []mode{modeImportSource, modeImportGroups} {
		m := newModel("pi", nil)
		m.mode = mode
		updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
		fm := updated.(model)
		if !fm.cancelled {
			t.Fatalf("mode %v not cancelled", mode)
		}
	}
}

func groupModeModel(groups []profileimport.Group) model {
	m := newModel("pi", nil)
	m.selected = "new-profile"
	m.mode = modeImportGroups
	m.selectedSource = profileimport.Source{Label: "src", Path: "/src"}
	m.available = groups
	m.selectedGroups = map[string]bool{}
	for _, group := range groups {
		m.selectedGroups[group.ID] = true
	}
	return m
}

func writeTUITestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}
