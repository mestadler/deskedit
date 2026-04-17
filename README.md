# deskedit

`deskedit` is a terminal UI for editing freedesktop.org `.desktop` entries.

It lets you browse installed application launchers, edit `[Desktop Entry]` keys (`Name`, `Exec`, `Icon`, `Terminal`, `NoDisplay`, `Hidden`), and toggle GPU offload without hand-editing files.

## Highlights

- Discovers entries from XDG app dirs (user + system).
- Preserves comments and key order when saving.
- Writes user overrides for system entries instead of editing `/usr/share` in place.
- Supports GPU `Exec=` toggles (switcheroo, NVIDIA PRIME, DRI_PRIME).
- Installs custom icons from `.png`, `.jpg`, `.gif`, or `.svg`.

## Build and run

Requires Go 1.22+.

```bash
make build    # produces ./deskedit
make run      # build + run
make check    # lint (gofmt+vet) then tests
make test     # go test ./...
make install  # install to $HOME/.local/bin (override with PREFIX=)
```

## Keybindings

List:
- `enter` open selected entry
- `/` filter by name or ID
- `q` / `esc` quit

Editor:
- `tab` / `shift-tab` switch fields
- `ctrl+i` open icon picker
- `ctrl+n` browse for custom icon (starts at `$HOME`)
- `ctrl+s` save
- `left` / `right` cycle GPU mode
- `esc` discard and go back

Install icon flow:
- select a file in browser, then confirm/edit icon name in form
- `ctrl+b` re-open browser from form

Browser screen:
- `enter` open directory or select file
- `esc` return to install form

## Caveats

- Only `[Desktop Entry]` keys are editable.
- `[Desktop Action ...]` groups are preserved but not shown in the UI.
- Flatpak-exported entries can be read, but saves still go to normal user overrides.

## License

MIT
