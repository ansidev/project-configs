# PC-3: TUI filter/selection — Space to select options, Enter to confirm

- **Ticket:** [PC-3] TUI filter/selection: Space to select options, Enter to confirm
- **Status:** Implemented
- **Date:** 2026-09-05
- **Branch:** `pc-3`

## Overview

The `project-configs` CLI previously used `pterm`'s interactive components for all
prompts. With filtering enabled, `pterm.DefaultInteractiveMultiselect` binds
**Enter** to toggle an option and **Tab** to confirm — the inverse of the expected
terminal convention. pterm cannot be reconfigured to the expected behaviour: it
explicitly rejects `keys.Space` as the select/confirm key when `Filter` is enabled
because Space is reserved for typing into the filter input.

The solution is to use `github.com/charmbracelet/huh v1.0.0` for the interactive
prompts. huh's `MultiSelect` natively binds **Space** to toggle option selection,
**Enter** to submit/confirm, and **`/`** to enter filter mode. pterm remains in the
project for all non-interactive styled output (success / error / info messages).

## Architecture

```
main.go
 ├─ promptProjectPath()        huh.Input        (validated, single-line)
 ├─ promptSelectConfigs()      huh.MultiSelect  (Space=toggle, Enter=confirm, /=filter)
 ├─ promptConfirm()            huh.Confirm      (Enter / ← → / y n)
 └─ CopyManager.CopyFilesConcurrently()
      └─ promptOverwrite()     huh.Confirm      (serialized by copyMu, non-aborting)
```

- `huh.NewForm(huh.NewGroup(...)).Run()` renders one field per screen, matching the
  previous numbered-prompt flow (`1.` path, `2.` configs, then confirm).
- huh renders to stderr by default (`tea.WithOutput(os.Stderr)`), so pterm messages
  printed to stdout never corrupt a prompt.
- Every prompt abort path is centralized in `runPrompt`, which maps
  `huh.ErrUserAborted` / `tea.ErrInterrupted` to a graceful cancellation message and
  exits with code 0.

## Components and Interfaces

### `prompt.go` (new)

| Function | Signature | Purpose |
| --- | --- | --- |
| `runPrompt` | `func runPrompt(field huh.Field, title string)` | Runs a single field standalone; on `huh.ErrUserAborted`/`tea.ErrInterrupted` prints a cancellation notice and exits 0. |
| `promptProjectPath` | `func promptProjectPath() string` | huh `Input` with title `1. Project path`, validated via `convertToFilePath`; invalid paths re-prompt inline. |
| `promptSelectConfigs` | `func promptSelectConfigs(options []string) []string` | huh `MultiSelect[string]` with title `2. Which configurations do you want to copy to your project?`; options are `huh.NewOption(label, label)`; returns selected labels. |
| `promptConfirm` | `func promptConfirm(message string) bool` | huh `Confirm` (`Affirmative("Yes")`, `Negative("No")`) with the given title. |
| `promptOverwrite` | `func promptOverwrite(dst string) bool` | huh `Confirm` for existing-file overwrite; returns `false` on abort (non-aborting). |

### `main.go` (modified)

- Prompt 1 → `promptProjectPath()` (replaces `pterm.DefaultInteractiveTextInput`).
- Prompt 2 → `promptSelectConfigs(getOptionLabels(configs))` (replaces
  `pterm.DefaultInteractiveMultiselect`).
- Final confirmation → `promptConfirm("Do you want to proceed?")` (replaces
  `pterm.DefaultInteractiveConfirm`).
- All `pterm.Error/Success/Info` output printing is unchanged.

### `copy.go` (modified)

- The per-file overwrite prompt inside `CopyFilesConcurrently` → `promptOverwrite(dst)`.
- The mutex is renamed `copyMu` (it serializes terminal access, not "confirmations").
- Aborting the overwrite prompt cancels only that file (event error
  `overwrite cancelled by user`), preserving current semantics.

## Data Models

No persisted data model changes. New in-memory mapping: config labels are wrapped as
`huh.NewOption(label, label)` so the multiselect's `[]string` value is directly
compatible with `getConfigMetadata(configs, selectedConfigs)`.

## Keybinding reference (huh v1.0.0 MultiSelect defaults)

| Key | Non-filtering mode | Filtering mode (`/`) |
| --- | --- | --- |
| `Space` (or `x`) | toggle option selection | typed into filter text |
| `Enter` | confirm/submit selection | apply filter, exit filter mode |
| `/` | enter filter mode | — |
| `Esc` | — | clear filter, exit filter mode |
| `↑/↓` (`k/j`) | move cursor | move cursor |
| `Ctrl+A` | select all / select none | select all |
| `Ctrl+C` | abort prompt (form-level quit) | abort prompt |

## Error Handling

| Scenario | Response |
| --- | --- |
| User aborts any top-level prompt (Ctrl+C) | `runForm` prints `pterm.Warning: Copying cancelled by user.` and exits 0. |
| User aborts an overwrite prompt | Only that file copy is cancelled (`overwrite cancelled by user` copy event); the run continues. |
| Invalid/empty project path | huh `Validate` shows an inline error; the prompt stays open (handled by the form, not os.Exit). |
| Config file unreadable/unparsable | unchanged: `pterm.Error` + exit 1. |

## Testing Strategy

- **Unit tests:** `prompt_select_configs_test.go` verifies that option labels are
  wrapped 1:1 into huh options preserving order and label=value mapping
  (`newConfigOptions` helper).
- **Build & static checks:** `go build ./...`, `go vet ./...` on macOS (darwin/arm64).
- **Interactive smoke test:** run the binary in a live terminal with a scripted
  keystroke sequence to verify Space toggles, Enter confirms, `/` filters, Esc aborts.
- **Regression check:** zero options selected still proceeds with an empty copy set.

## Decision: replace pterm interactive components with huh

**Context:** The ticket requires Space = select option and Enter = confirm in the
filter/selection UI. pterm v0.12.83 hard-codes Enter = select, Tab = confirm, and
returns an error if Space is configured for select/confirm while `Filter` is enabled.

**Options Considered:**

1. **Stay on pterm, disable `WithFilter`** — Pros: no new dependency / Cons: loses
   filtering entirely, and Space would still not toggle (Enter would) — fails the ticket.
2. **Stay on pterm, fork/patch the multiselect printer** — Pros: no new dependency /
   Cons: maintaining a fork of a UI printer for a keybinding change; high upkeep.
3. **Switch interactive prompts to `charmbracelet/huh v1.0.0`** — Pros: native
   Space-toggle/Enter-confirm/`/`-filter UX matching the ticket exactly, stable v1 API,
   actively maintained by Charm; pterm kept for styled output / Cons: new transitive
   dependency tree (bubbletea, lipgloss), slightly different visual style.

**Decision:** Option 3 — adopt huh v1.0.0 for all interactive prompts.

**Rationale:** Only huh natively satisfies the required keybinding model together with
filtering; it is a stable v1.0.0 release and the migration surface is limited to the
three prompt call sites plus the overwrite confirm.
