# AGENTS.md

## Overview

HoneyberriDB is a single-file Go CLI key-value store (`main.go`, stdlib only): append-only WAL in the working directory, in-memory index, JSON snapshot, and history export. There is no `go.mod`, test suite, CI, or lint config.

## Commands

- Build: `go build -o /tmp/honeyberri main.go`. `go build .` fails because there is no `go.mod`.
- Run: `go run main.go <write|read|history> ...`
  - `write` / `w <tapari> <duna> <key> <value...>` — value args are joined with spaces
  - `read` / `r` — print current state from memory
  - `history` / `h` — print every value per key and write a history file
- Format/vet: `gofmt -w main.go`, `go vet main.go`.
- No tests; verify changes with a build plus a CLI run in a scratch directory.

## Data files

- All state lives in the current working directory. Running commands in the repo root creates or modifies tracked data files; test in a temp dir instead:
  `mkdir -p /tmp/honey && cp main.go /tmp/honey && cd /tmp/honey && go run main.go write ...`
- Filenames derive from `WALversion` / `SoftwareVersion` at the top of `main.go`. Current live files are `HoneyberriDB0.0.1-alpha.3.aakku`, `Snapshot.HoneyberriDB0.0.1-alpha.3.aakku`, and `History.HoneyberriDB0.0.1-alpha.3.aakku`.
- Every other `.aakku` file and `data.json` are historical; current code does not read them. In particular, `snapShot.aakku` is not the snapshot file.
- Bumping either version constant switches to a fresh dataset; there is no migration. Old WAL timestamp formats are deliberately unsupported — only unix-nano records parse.
- Preserve the WAL line format (`\n` + space-separated header + `;;value;;` + nanos + FNV-128a hash) and snapshot format (`<json>\n<checksum>`). Bump the version constants when changing either format.
- Header fields must be non-empty, whitespace-free, and must not contain `;;`; values cannot contain line breaks.

## Implementation notes

- Write order: `Set` takes `db.mu`, appends and fsyncs to the WAL, then updates memory. `OpenDb` loads the snapshot, then replays WAL records with seq above it. `CloseDb` writes a snapshot only if seq advanced.
- Lock order must remain `db.mu -> wal.mu`; never acquire `db.mu` while holding `wal.mu`. The WAL uses a `Mutex` because scanning seeks the shared file offset.
- `main` at the repo root is a checked-in arm64 binary; do not overwrite it — build to `/tmp`.
- The Git remote is named `HoneyberriDB`, not `origin`.
