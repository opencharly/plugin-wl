# AGENTS.md — plugin-wl

Standalone plugin repo for the `wl` live-container check verb (`verb:wl`). The
plugin is a Go module at `candy/plugin-wl/` (module path
`github.com/opencharly/plugin-wl/candy/plugin-wl`); the root `charly.yml`
declares `discover: candy` **and** the `wl-skill` + `wl-overlay-skill` `skill:`
entities (the corpus sources for `/charly-check:wl` and `/charly-check:wl-overlay`).

Canonical files:

- `candy/plugin-wl/charly.yml` — the `plugin-wl:` candy entity (`plugin:` block,
  `plan:` check) + the two skill entities.
- `candy/plugin-wl/methods.go` — the method surface.
- `candy/plugin-wl/compositor.go` — the per-compositor backend routing
  (`detectCompositor`).
- `candy/plugin-wl/hyprland_layers.go` — the `hypr-layers` geometry correlation.
- `candy/plugin-wl/atspi.go` — the AT-SPI2 accessibility methods.
- `candy/plugin-wl/schema/wl.cue` — the self-contained `#WlInput`.
- `.github/workflows/tag-on-merge.yml` — CalVer tag + `CHANGELOG/` on merge.
- `README.md` — user overview only; never agent guidance.

## Load these skills first (R0)

- `/charly-internals:plugin` — the plugin authoring reference: the `plugin:`
  block, the unified Provider model, the out-of-process/EXEC shape, the
  per-plugin CUE-schema contract, placement. Load before touching the provider or
  schema.
- `/charly-check:wl` — the `wl:` verb this candy serves.
- `/charly-check:wl-overlay` — the `wl: overlay-*` methods this candy also serves.
- `/charly-check:check` — the check orchestrator, beds and the R10 sequence.
- `/charly-internals:git-workflow` — before any git/PR action.

## Build / validate / test

- `go build ./...` in `candy/plugin-wl/` — compile the plugin module.
- `go test ./...` in `candy/plugin-wl/` — the plugin's Go tests (the methods,
  compositor routing, Hyprland layers, AT-SPI, and the OCR/artifact validators).
- `charly box validate` at the repo root — the structural check (the candy +
  `plugin:` block, CUE schema, the skill entities).
- The merge gate is the **org-wide** `charly/pr-validator` (required check
  `validate / validate`, defined in `opencharly/.github`); this repo has **no**
  per-repo candy gate.
- R10 consumer: a desktop pod bed whose check composes this plugin (the
  `sway-browser-vnc` bed's `wl` sway-tree + screenshot probes).

## Modify this repo

- Edit the `plugin-wl:` candy entity, the Go source, and `schema/wl.cue`
  **together** — the schema is the single source for the `params/` struct, so a
  field change not mirrored in the schema desyncs the generated types.
- The skill entities are the corpus sources for `/charly-check:wl` and
  `/charly-check:wl-overlay`; a change to the verb surface belongs in BOTH the
  candy and the skill body.
- Keep the plugin **compositor-aware**: a method with no backend on a compositor
  must fail fast with a clear error, never hang.

## Landing

- PR-only. Every change lands through a pull request; the org-required
  `charly/pr-validator` validates the diff and body and arms native auto-merge on
  PASS. Direct pushes to `main` are blocked.
- History lives in `CHANGELOG/` (written by `tag-on-merge` at merge time); the PR
  body IS the changelog.
- The authoritative rulebook is the umbrella `AGENTS.md` in
  `opencharly/opencharly` and `charly/AGENTS.md` in the charly repo. Do not
  restate its rules here.
