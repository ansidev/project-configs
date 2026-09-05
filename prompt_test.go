package main

import (
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/huh"
)

// newTestMultiselect builds the same multiselect used by promptSelectConfigs.
// The default keymap must be injected explicitly because huh normally does so
// when the field is wrapped in a form (huh.NewForm).
func newTestMultiselect(labels []string) *huh.MultiSelect[string] {
	m := huh.NewMultiSelect[string]().
		Title("2. Which configurations do you want to copy to your project?").
		Options(newConfigOptions(labels)...).
		Height(6)
	field := m.WithKeyMap(huh.NewDefaultKeyMap())
	updated, ok := field.(*huh.MultiSelect[string])
	if !ok {
		panic("WithKeyMap did not return the multiselect field")
	}
	return updated
}

// sendKeys feeds key messages to the multiselect model.
func sendKeys(t *testing.T, m *huh.MultiSelect[string], keyMsgs ...tea.KeyMsg) {
	t.Helper()
	for _, msg := range keyMsgs {
		model, _ := m.Update(msg)
		updated, ok := model.(*huh.MultiSelect[string])
		if !ok {
			t.Fatalf("Update returned unexpected model type %T", model)
		}
		*m = *updated
	}
}

func keyString(s string) tea.KeyMsg {
	return tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(s)}
}

// selectedValues returns the multiselect value as a typed string slice.
func selectedValues(t *testing.T, m *huh.MultiSelect[string]) []string {
	t.Helper()
	got, ok := m.GetValue().([]string)
	if !ok {
		t.Fatalf("GetValue returned %T, want []string", m.GetValue())
	}
	return got
}

// TestMultiselectSpaceTogglesAndEnterConfirms verifies the core ticket
// requirement: Space toggles an option without submitting; Enter confirms the
// selection.
func TestMultiselectSpaceTogglesAndEnterConfirms(t *testing.T) {
	labels := []string{"git-flow for GitHub", ".editorconfig", "MIT LICENSE"}
	m := newTestMultiselect(labels)

	// Cursor starts on the first option. Space must toggle it, not submit.
	sendKeys(t, m, keyString(" "))

	selected := selectedValues(t, m)
	if len(selected) != 1 || selected[0] != "git-flow for GitHub" {
		t.Fatalf("after Space expected [git-flow for GitHub], got %v", selected)
	}

	// Toggling again with Space must deselect (no submission in between).
	sendKeys(t, m, keyString(" "))
	if got := selectedValues(t, m); len(got) != 0 {
		t.Fatalf("after second Space expected empty selection, got %v", got)
	}

	// Move down twice and select "MIT LICENSE".
	sendKeys(t, m,
		tea.KeyMsg{Type: tea.KeyDown},
		tea.KeyMsg{Type: tea.KeyDown},
		keyString(" "),
	)
	selected = selectedValues(t, m)
	if len(selected) != 1 || selected[0] != "MIT LICENSE" {
		t.Fatalf("after down,down,Space expected [MIT LICENSE], got %v", selected)
	}
}

// TestMultiselectEnterDoesNotToggle verifies Enter confirms/submits and never
// toggles an option (opposite of the previous pterm behaviour).
func TestMultiselectEnterDoesNotToggle(t *testing.T) {
	labels := []string{"a", "b", "c"}
	m := newTestMultiselect(labels)

	sendKeys(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := selectedValues(t, m); len(got) != 0 {
		t.Fatalf("Enter must not toggle options, got %v", got)
	}
}

// TestMultiselectFilterModeSpaceTypesText verifies that after pressing "/" the
// field enters filter mode where Space is typed into the filter (not used to
// toggle options), and Esc leaves filter mode.
func TestMultiselectFilterModeSpaceTypesText(t *testing.T) {
	labels := []string{"git-flow for GitHub", ".editorconfig", "MIT LICENSE"}
	m := newTestMultiselect(labels)

	// Enter filter mode.
	sendKeys(t, m, keyString("/"))
	if !m.GetFiltering() {
		t.Fatal("expected multiselect to be in filtering mode after '/'")
	}

	// Type "git" then Space: Space must go into the filter text.
	sendKeys(t, m, keyString("g"), keyString("i"), keyString("t"), keyString(" "))

	// Leave filter mode with Enter (SetFilter); selection must be unchanged.
	sendKeys(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	if got := selectedValues(t, m); len(got) != 0 {
		t.Fatalf("filtering keys must not toggle options, got %v", got)
	}
	if m.GetFiltering() {
		t.Fatal("Enter should leave filtering mode")
	}
}

// TestMultiselectFilterNarrowsOptions verifies that an applied filter narrows
// the option list to matching entries and that after leaving filter mode Space
// toggles the filtered option.
func TestMultiselectFilterNarrowsOptions(t *testing.T) {
	labels := []string{"git-flow for GitHub", ".editorconfig", "MIT LICENSE"}
	m := newTestMultiselect(labels)

	sendKeys(t, m, keyString("/"))
	sendKeys(t, m, keyString("e"), keyString("d"), keyString("i"), keyString("t"))
	sendKeys(t, m, tea.KeyMsg{Type: tea.KeyEnter})

	// The filtered list must be navigable to the single match and toggleable.
	sendKeys(t, m, keyString(" "))
	if got := selectedValues(t, m); len(got) != 1 || got[0] != ".editorconfig" {
		t.Fatalf("expected [.editorconfig] after filter+Space, got %v", got)
	}
}

// TestDefaultKeymapDocumentsTicketKeybindings pins the huh keymap bindings the
// ticket depends on, so a future huh upgrade that changes them is caught.
func TestDefaultKeymapDocumentsTicketKeybindings(t *testing.T) {
	km := huh.NewDefaultKeyMap()

	toggleKeys := km.MultiSelect.Toggle.Keys()
	if !contains(toggleKeys, " ") {
		t.Errorf("MultiSelect.Toggle must bind Space, got %v", toggleKeys)
	}
	if !contains(km.MultiSelect.Submit.Keys(), "enter") {
		t.Errorf("MultiSelect.Submit must bind enter, got %v", km.MultiSelect.Submit.Keys())
	}
	if !contains(km.MultiSelect.Filter.Keys(), "/") {
		t.Errorf("MultiSelect.Filter must bind /, got %v", km.MultiSelect.Filter.Keys())
	}
}

func contains(keys []string, want string) bool {
	for _, k := range keys {
		if k == want {
			return true
		}
	}
	return false
}
