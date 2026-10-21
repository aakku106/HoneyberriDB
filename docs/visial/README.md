# HoneyberriDB function-call visuals

An offline, interactive map of the function calls in `main.go` for
**HoneyberriDB 0.0.1-alpha.3**.

Open `FuncCalls_Visuals.0.0.1-alpha.3.html` directly in a browser. No server,
build step, or network connection is required.

## Views

- **Runtime flow** — the boot → dispatch → shutdown path for `write`, `read`,
  and `history`, with the write/read/history lanes separated.
- **Call tree** — every project-internal call rooted at `main`. Functions that
  are called from multiple places appear under each caller; the detail panel
  lists all direct callers.
- **Function index** — all 27 callables with role, phase, call counts, caller
  counts, source line ranges, and flags. Sortable and filterable.

Click any function name to open its signature, summary, project calls, callers,
standard-library calls, and implementation notes. The **Show in call tree**
action jumps to and highlights that function.

## Files

| File | Purpose |
| --- | --- |
| `FuncCalls_Visuals.0.0.1-alpha.3.html` | Page structure and static runtime-flow diagram |
| `func-calls.css` | Theme, layout, responsive styles, print styles |
| `call-graph-data.js` | Call graph and per-function metadata extracted from `main.go` |
| `func-calls.js` | Tabs, tree, filtering, index, theme, and detail dialog |
| `README.md` | This file |

The graph data is a JavaScript file rather than a JSON file so the page works
when opened with `file://` — browsers block `fetch()` of local JSON.

## Graph snapshot

- 27 callables: 15 functions (including 2 generic helpers) and 12 methods
- 31 project-internal call edges
- Maximum call depth: 6 edges from `main`
- `Get` is present in the source but unused by the CLI
- Standard-library calls are annotated per function but are not drawn as nodes

The source link in the header and detail panel points to `../../main.go`.

## Regenerating

The visuals are a point-in-time snapshot. When `main.go` changes, update
`call-graph-data.js` (IDs, signatures, line ranges, calls, notes) and the static
flow markup in the HTML if the command paths changed. Also bump the version in
the filename and page copy if `SoftwareVersion` changes.

Generated 2026-10-22 03:49:58 NPT.
