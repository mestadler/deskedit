# AGENTS

## Fast facts
- Single Go module (`go.mod`), Go 1.22, CLI/TUI app.
- Entry point is `main.go` -> `internal/ui.Run()`.
- Core packages:
  - `internal/ui`: Bubble Tea screens/state machine.
  - `internal/desktop`: `.desktop` discovery, parser/serializer, atomic saves, icon install.
  - `internal/gpu`: GPU capability detection and `Exec=` wrap/unwrap logic.

## Verified developer commands
- Build: `make build` (writes binary `./deskedit`).
- Run app: `make run`.
- Local quality gate: `make check` (runs `lint` then `test`, fail-fast).
- Full tests: `make test` (same as `go test ./...`).
- Focused tests:
  - `go test ./internal/gpu -run TestDetectMode`
  - `go test ./internal/desktop -run TestParseRoundTrip`
  - `go test ./internal/desktop -run TestDiscover`
  - `go test ./internal/ui -run TestSave_SystemEntryWritesUserOverride`
  - `go test ./internal/ui -run TestCtrlNOpensInstallBrowserFromHome`
  - `go test ./internal/ui -run TestUpdate_HandlesEntriesRefreshedMsg`
  - `go test ./internal/ui -run TestRegionFocus_TraversalCyclesHeaderBodyFooter`
  - `go test ./internal/ui -run TestFooterRegion_SaveActionParityWithKeyAndPalette`
  - `go test ./internal/ui -run TestSaveConfirm_YesWritesAndReturnsToList`
  - `go test ./internal/ui -run TestExitConfirm_YesQuits`

## Lint/format behavior
- `make lint` is intentionally strict: it fails if `gofmt -l .` returns any files, then runs `go vet ./...`.
- Use `gofmt -w .` before `make check` to avoid avoidable lint failures.

## Behavior constraints worth preserving
- `.desktop` parser is order/comment-preserving; unmodified files should round-trip unchanged (`internal/desktop/parser.go`, `parser_test.go`).
- Edits are only for `[Desktop Entry]`; do not mutate other groups unless intentionally extending behavior.
- The editor currently treats `Terminal`, `NoDisplay`, and `Hidden` as first-class `[Desktop Entry]` fields (boolean toggles, saved as `true`/`false`).
- Saves are atomic (`tmp + rename`) in both desktop-file save and icon install paths.
- System desktop entries are not edited in place; edits should write user overrides (`internal/desktop/discover.go` + UI save flow).
- GPU mode toggles must stay idempotent; use `gpu.Wrap`/`gpu.Unwrap` semantics instead of stacking prefixes manually.
- NVIDIA env-prefix parsing must keep handling mixed-case var names (for `__VK_LAYER_NV_optimus`) so mode switches fully unwrap old prefixes.
- Icon install flow is browser-first (`ctrl+n` opens browser at `$HOME`/last dir), then path+name confirmation form.
- UI key handling is centralized via Bubble `key.Binding` maps and rendered with Bubble `help.Model`; keep bindings and help text in one source of truth.
- Shell layout is intentionally framed into header/body/footer regions inside a stable outer frame; avoid reintroducing screen-specific layout jumps.
- Region focus traversal uses `ctrl+tab`/`ctrl+shift+tab`; keep body-local `tab` behavior intact for form/navigation fields.
- Footer primary bindings render as selectable chips; when footer is focused, `tab`/`shift+tab` cycle chips and `enter` executes the selected action via existing command paths.
- Confirmation modal is a shared red-styled guardrail for destructive actions; keep options strictly `Yes`/`No`, default selection `No`, and `esc` mapped to `No`.
- Exit and save flows are confirm-gated; preserve current behavior that only `Yes` executes quit/save while `No` leaves state unchanged.
- Custom tea messages used by commands must always have matching `Update` handlers (notably list-refresh after save).
- Keep `internal/ui` split by screen concerns (`screen_list.go`, `screen_editor.go`, `screen_icon_picker.go`, `screen_install.go`) rather than growing a single monolith file.

## Environment/side effects
- Discovery and writes depend on XDG env vars (`XDG_DATA_HOME`, `XDG_DATA_DIRS`) with XDG defaults when unset.
- Icon install writes under `$XDG_DATA_HOME/icons/hicolor` and *best-effort* runs `gtk-update-icon-cache`; absence/failure is non-fatal.
