# mdv: terminal markdown annotation tool

A markdown viewer for the terminal, designed with agentic workflows in mind: open a `.md` file, scroll, leave notes anchored to headings, exit. Notes land in a sibling `<doc>.notes.md` next to the source.

## Build & Test

```sh
go build ./...   # compile
go test ./...    # unit tests (notes save/reload + heading paths)
go install .     # install to $GOPATH/bin/mdv
```

Go 1.24+. No Makefile, no CI.

## Architecture

```
main.go              # argv → tui.Run(path)
tui/
  run.go             # read file, detect terminal size, prewarm glamour, launch Bubble Tea
  model.go           # Bubble Tea Model + Update + View. Cursor, scroll, tabs, search, help.
  markdown.go        # parse H1–H6 source headings, glamour render, locate rendered line offsets
  compose.go         # textarea overlay for entering a note
  notes.go           # NoteStore: sibling .notes.md, saved on ctrl+s and reloaded on next open
  styles.go          # lipgloss style definitions
```

Six files, no `internal/`, no submodules.

## Conventions

**Anchor model.** Notes are anchored by heading path: `Anchor{Path: []string}`. Empty path = document-level. Paths come from walking the H1–H6 source headings and finding ancestors with strictly lower level.

**Sibling file is the source of truth.** `<doc>.notes.md` is plain markdown that `mdv` reads back on next open, so the file on disk is also the working draft. No separate yaml store, no clipboard handoff. `formatNotesFile`/`parseNotesFile` are inverses; the test in `notes_test.go` pins this.

**3-column gutter on every line.** `[cursor][note][ ]`. Reserved on every rendered line so cursor motion never shifts content horizontally. Glamour is rendered at `width - gutterWidth` so the gutter doesn't squeeze the content. See `applyMarkers` in `model.go`.

**Cursor with scrolloff.** `j`/`k` move an explicit `cursorLine`; viewport only follows when the cursor crosses within `scrollOff = 5` of an edge. `[`/`]`/`g`/`G` snap the cursor with `snapViewportToCursor` instead of the lazy `adjustViewportToCursor`.

**Tab bar = H2 outline.** Only H2s appear as tabs. H3/H4 are reached by scrolling. The cursor still anchors notes against whichever heading it's under, regardless of level.

**Terminal width is detected before Bubble Tea starts** (via `golang.org/x/term` in `run.go`) so glamour can prewarm at the real width. Without this the first frame is a black flash while glamour rebuilds at the right size.

## Deliberately skipped

These were considered and left out; don't reflexively add them.

- **fsnotify / file watcher.** Source edits while open won't refresh. Add only if it becomes a real pain. Review sessions are short.
- **Directory mode.** Single file only. Sidebar of multiple files would be a meaningful new shape; revisit if needed.
- **Inline substring highlight on search.** Matches are line-level (cursor jumps to the matched line). Per-match ANSI position math is non-trivial; defer until line-level proves insufficient.
- **Clipboard submit / draft yaml.** specv-style submit flow was deliberately dropped; the sibling `.notes.md` IS the artifact.
- **Shared library with flashspec/specv.** mdv was inspired by them but is independent. Don't refactor into shared packages.

## Commit messages

Conventional commits: `<type>(<scope>): <subject>`. Types: `feat`, `fix`, `chore`, `docs`, `refactor`, `test`. Scopes are optional; if used, prefer `tui`, `notes`, `markdown`, `compose`. Examples:

- `feat(tui): add ? help overlay`
- `fix(notes): preserve blank lines in note body`
- `refactor(tui): cursor-driven heading anchoring`
