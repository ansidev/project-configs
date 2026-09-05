# PC-6: Restructure config.yaml — label/resources schema with snake_case ids

- **Ticket:** [PC-6] Restructure config.yaml: label/resources schema with snake_case ids
- **Status:** Implemented
- **Date:** 2026-09-05

## Overview

`config.yaml` previously used human-readable top-level keys (`"git-flow for GitHub"`,
`"MIT LICENSE"`, ...) that mapped directly to an array of resource entries. This
coupled the group key, display label, and data shape in one mechanism, offered no
stable identifiers, and made programmatic references impossible.

The new schema keeps a top-level map, but each key is now the **snake_case id of the
group** and each value has a `label` (human-readable) and `resources` (array). Every
resource has a required snake_case `id`.

## Data Models

```go
type configResource struct {
    ID          string `yaml:"id"`
    Path        string `yaml:"path"`
    Target      string `yaml:"target,omitempty"`
    PostMessage string `yaml:"post_message"`
}

type configGroup struct {
    Label     string           `yaml:"label"`
    Resources []configResource `yaml:"resources"`
}
```

`loadConfig` now parses `map[string]configGroup` and validates:

- group key must match `^[a-z][a-z0-9_]*$`
- `label` must be non-empty
- `resources` must be non-empty
- each resource `id` must be non-empty snake_case and unique within its group

Selection is wrapped in `configOption{ID, Label}` so the huh multiselect displays
the `label` while its `[]string` value carries the group `id` used by
`getConfigMetadata`.

## Migration mapping

| Old top-level key | New key (group id) | label |
| --- | --- | --- |
| `git-flow for GitHub` | `git_flow_for_github` | `git-flow for GitHub` |
| `changelog generator` | `changelog_generator` | `changelog generator` |
| `.editorconfig` | `editorconfig` | `.editorconfig` |
| `renovate.json` | `renovate_json` | `renovate.json` |
| `MIT LICENSE` | `mit_license` | `MIT LICENSE` |
| `GitHub Funding` | `github_funding` | `GitHub Funding` |

Resource ids are derived from the resource path (snake_case of basename without
extension), e.g. `.github/workflows/create_release_pr.yaml` → `create_release_pr`.

## Components and Interfaces

| Function | Before | After |
| --- | --- | --- |
| `loadConfig` | `(map[string][]configMetadata, error)` | `(map[string]configGroup, error)` + validation |
| `getOptionLabels` | `[]string` labels | `[]configOption` (id + label), deterministic order (sorted by group id) |
| `getConfigMetadata` | lookup by label | lookup by group id, returns flattened `[]configResource` |
| `newConfigOptions` | `huh.NewOption(label, label)` | `huh.NewOption(option.Label, option.ID)` |
| `promptSelectConfigs` | returns selected labels | returns selected group ids |

`copy.go` is unchanged: it consumes `[]configResource` (same `path`/`target`/
`post_message` semantics).

## Error Handling

| Scenario | Response |
| --- | --- |
| Group key not snake_case | loadConfig error: `group id %q must be snake_case` |
| Missing/blank `label` | loadConfig error per group |
| Empty `resources` | loadConfig error per group |
| Missing/blank/non-snake_case resource `id` | loadConfig error: `resource id must be snake_case` |
| Duplicate resource `id` in a group | loadConfig error: `duplicate resource id %q in group %q` |

## Testing Strategy

- `config_test.go`: updated for the new schema; new table-driven
  `TestLoadConfigValidation` covers missing id, non-snake_case id, duplicate id,
  missing label/resources, non-snake_case group key (using temp YAML files).
- `prompt_test.go`: option wrapping verified against id/label pairs; selection
  values are group ids.
- Regression: `go build ./...`, `go vet ./...`, `go test ./...`.

## Decision: map keyed by snake_case group id (not top-level list)

**Context:** Requirement 4 was ambiguous about the new top-level shape.

**Options Considered:**

1. Top-level map keyed by snake_case id — minimal diff to lookup semantics.
2. Top-level list of groups with per-group `id` — fixes map ordering but changes
   lookup code more broadly.

**Decision:** Option 1, confirmed by the user.

**Rationale:** Keeps `getConfigMetadata` map-lookup semantics and minimizes the
change surface; deterministic ordering is handled by sorting ids when building
prompt options.
