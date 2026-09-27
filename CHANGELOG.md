# Changelog

All notable changes to this project are documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

## [0.3.2] - 2026-09-27

### Security

- **Escape sequences in the error banner.** Filesystem errors embed the
  offending path, and the error banner rendered them raw, so a directory whose
  name carried ANSI/OSC sequences reached the terminal through the banner even
  though the entry list was already sanitized. The banner now goes through the
  same control-character sanitization as every other render path.
- Release binaries are now built with Go 1.25.14 (`toolchain` directive) to pick
  up the fix for GO-2026-4602 in `os`. The minimum Go version for importers is
  unchanged at 1.25.0.

### Changed

- Dependency updates: `sahilm/fuzzy` v0.1.3, `golang.org/x/term` v0.45.0, and
  indirect updates to `x/sys`, `x/text`, `charmbracelet/x/ansi`,
  `go-runewidth`, `go-isatty`, `go-colorful` and `xo/terminfo`.
- CI actions bumped: `actions/checkout` v7, `actions/setup-go` v6.5.0,
  `codecov/codecov-action` v7, `golangci-lint-action` v9.3.0,
  `goreleaser-action` v7.2.3.

## [0.3.1] - 2026-05-29

### Changed

- README demo recordings (VHS).
- CI actions bumped: `codecov/codecov-action` v6, `goreleaser-action` v7.

## [0.3.0] - 2026-05-29

### Changed

- **Renamed `go-finder` → `rummage`.** The module path is now
  `github.com/rummage-dev/rummage` (previously
  `github.com/SREsAreHumanToo/go-finder`). The package name is unchanged
  (`finder`), so the public API is identical — only the import path changes:

  ```go
  import finder "github.com/rummage-dev/rummage"
  ```

  `finder.PickFile()` and every other call work exactly as before. Existing
  users on the old path continue to resolve via GitHub's repository redirect.

### Security

- **Terminal escape-sequence injection** — untrusted filenames and previewed
  file contents are now stripped of control characters (C0/C1/DEL, including the
  `ESC` that prefixes every ANSI/OSC sequence) before being rendered. Previously
  a crafted name or file in a browsed directory could emit raw escape sequences
  to the terminal (e.g. rewrite the window title via OSC 0, manipulate the
  cursor to spoof the UI, or hijack the clipboard via OSC 52). A custom
  `PreviewFunc` is trusted caller code and is left unsanitized. `validateName`
  also now rejects control characters in interactively created names.

### Fixed

- Breadcrumb and filename truncation are now rune/display-width aware, so a
  multibyte name is never sliced mid-rune into invalid UTF-8.

### Added

- **Extension filter** — `WithExtensions("pdf", "docx", "doc")` limits the
  picker to a set of file extensions, matched case-insensitively (so
  `Report.PDF` is included). Accepts values with or without a leading dot.
  Composes with `WithFilter`: a file is shown if it matches either. This is the
  ergonomic, case-correct path for the common "only allow these document types"
  use case, where the case-sensitive `WithFilter("*.pdf")` glob falls short.

## [0.2.0] - 2026-05-28

The biggest feature release yet: rummage goes from a one-line picker to the
batteries-included **and** embeddable terminal picker for Go, while staying fully
backward-compatible. Existing `PickFile`/`PickFolder`/`PickAny`/`PickMultiple`
code keeps working unchanged.

### Added

- **Fuzzy search** — `/` filters with scored fuzzy matching (via `sahilm/fuzzy`),
  ranked best-match-first, with matched characters highlighted as you type. Opt
  out with `WithFuzzySearch(false)` for plain substring matching.
- **Pluggable filesystem** — `WithFS(fs.FS)` browses any `io/fs.FS` (e.g.
  `embed.FS`, `fstest.MapFS`), not just the host OS. Introduces the `FileSystem`
  and `WritableFileSystem` interfaces. Custom filesystems are read-only, so
  interactive create/delete degrades gracefully.
- **Preview pane** — opt-in `WithPreview(true)` shows a side pane previewing the
  highlighted entry (file head, directory listing, or metadata). Customizable via
  `WithPreviewFunc` and the new `PreviewFunc` type.
- **Embeddable component** — `New(...Option)` returns a Bubble Tea sub-model you
  can nest in your own program; it reports completion via `DoneMsg` and `State()`
  (with `Done()`) instead of quitting. Also adds `WithEmbedded`.
- New styles: `Styles.Preview`, `Styles.PreviewBorder`, `Styles.Match`.
- New examples: `examples/preview` and `examples/embedded`; a `-preview` flag on
  `examples/basic`.
- README "Why rummage?" comparison table and `docs/POSITIONING.md`.

### Fixed

- Preview rendering no longer jumps or corrupts the display while scrolling:
  list and preview columns are fixed-height, the list is sized from the actual
  (wrap-aware) help-bar height, and a new `clampView` hard-limits every frame to
  the terminal width × height so no line can wrap and the frame can never
  overflow the screen at any terminal size.

### Compatibility

No breaking changes. Requires Go 1.25+.

## [0.1.2] - 2026-05-27

- Packaging and tooling: `doc.go` with runnable godoc examples, `.golangci.yml`,
  `SECURITY.md`, GoReleaser, and Codecov; CI test step uses bash on Windows.

## [0.1.1] - 2026-05-05

- Maintenance and dependency updates.

## [0.1.0] - 2026-03-11

- Initial release: cross-platform terminal file/folder picker built on Bubble
  Tea. File, folder, any, and multi-select modes; live search; interactive
  create/delete; hidden-file toggle; symlink expansion; WSL path helpers; and
  fully customizable keybindings and styles.

[Unreleased]: https://github.com/rummage-dev/rummage/compare/v0.3.2...HEAD
[0.3.2]: https://github.com/rummage-dev/rummage/compare/v0.3.1...v0.3.2
[0.3.1]: https://github.com/rummage-dev/rummage/compare/v0.3.0...v0.3.1
[0.3.0]: https://github.com/rummage-dev/rummage/compare/v0.2.0...v0.3.0
[0.2.0]: https://github.com/rummage-dev/rummage/compare/v0.1.2...v0.2.0
[0.1.2]: https://github.com/rummage-dev/rummage/compare/v0.1.1...v0.1.2
[0.1.1]: https://github.com/rummage-dev/rummage/compare/v0.1.0...v0.1.1
[0.1.0]: https://github.com/rummage-dev/rummage/releases/tag/v0.1.0
