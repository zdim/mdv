# mdv

Terminal markdown viewer with inline notes. Open any `.md` file, scroll around, leave notes anchored to headings. Notes land in a sibling `<doc>.notes.md` file so an agent or collaborator can read them back.

## Install

```sh
go install github.com/zdim/mdv@latest
```

## Usage

```sh
mdv path/to/doc.md
```

Notes are written to `<doc>.notes.md` next to the source; delete that file to start fresh. Press `?` in the viewer for the full keybinding reference.

## Notes file format

Markdown, one section per anchor with the heading path as the section title:

```markdown
# Notes on plan.md

## Implementation / Phase 2 / Database changes

Should we shard before partitioning?

## Implementation / Phase 2 / Migration

Timeline is tight.
```

The format is round-trippable — `mdv` parses it back on next open so you can keep editing.
