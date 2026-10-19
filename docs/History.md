# HoneyberriDB — Project History

| | |
|---|---|
| Repository | `HoneyberriDB` (`git@github.com:aakku106/HoneyberriDB.git`) |
| Documented revision | `d434d69` — 2026-10-19 23:59:51 NPT |
| History window | 2026-10-12 00:46:49 → 2026-10-19 23:59:51 NPT |
| Commits analysed | 141 commits on `main` (including 1 merge); 3 tags |
| Timezone | Asia/Kathmandu (NPT, UTC+05:45) |
| Primary source | Git history, `main.go`, tag metadata, committed data files |
| Document date | 2026-10-20 |

This document reconstructs the complete engineering history of HoneyberriDB from its Git history. It explains what was built, why the implementation changed, what each change enabled, and what defects or risks it introduced. Original commit messages are preserved verbatim in Appendix A; the prose paraphrases them and labels inferences where the repository does not provide direct evidence.

## Contents

1. [Scope and Method](#1-scope-and-method)
2. [Executive Summary](#2-executive-summary)
3. [Release and Milestone Timeline](#3-release-and-milestone-timeline)
4. [Phase 0 — Flat-File Prototype](#4-phase-0--flat-file-prototype-v001-alpha0)
5. [Phase 1 — WAL Identity and Locking](#5-phase-1--wal-identity-and-locking-v001-alpha1)
6. [Phase 2 — WAL v2](#6-phase-2--wal-v2-structured-records-and-checksums-001-alpha2)
7. [Phase 3 — Hardening and PR #1](#7-phase-3--hardening-and-pr-1-001-alpha2-release-line)
8. [Phase 4 — In-Memory Database and Snapshots](#8-phase-4--in-memory-database-and-snapshots)
9. [Phase 5 — Alpha.3 Consolidation and Documentation](#9-phase-5--alpha3-consolidation-and-documentation)
10. [Defect and Risk Register](#10-defect-and-risk-register)
11. [Architecture at HEAD](#11-architecture-at-head)
12. [Engineering Lessons](#12-engineering-lessons)
- Appendix A — Complete Commit Index
- Appendix B — Tags, Branches, and Artifacts
- Appendix C — Method, Classification, and Caveats

---

## 1. Scope and Method

The analysis covers every commit reachable from `main` at `d434d69`:

- `git log --reverse -p` was read in full, commit by commit.
- Merge metadata, tag objects, branch refs, and the stash were inspected separately.
- The current `main.go` (587 lines) was read as the final state and cross-checked against the code introduced in each commit.
- Every commit's `main.go` was extracted into a temporary directory and compiled with the current Go toolchain. Sixteen commits failed to build; the windows and causes are catalogued in Section 10.4.
- Committed `.aakku`, `.json`, and binary artifacts were inspected as historical fixtures.
- A defect is marked **confirmed** only when a later commit visibly corrects it. A **potential** defect is a code-level concern that was not necessarily exercised at runtime.
- The repository has no test suite, CI, benchmarks, or runtime telemetry. Consequently, "impact" describes code behaviour and logical consequences, not measured production incidents. Historical bug windows may never have been hit in practice.

The working tree was clean at `d434d69` when this document was written. Dates and times are author dates in NPT, the timezone recorded in every commit.

---

## 2. Executive Summary

HoneyberriDB is a single-file, standard-library-only Go CLI key-value store. Its history is an eight-day, 141-commit record of a project that started as a personal data file and progressively acquired the mechanisms of a small database: an append-only write-ahead log (WAL), checksums, replay, history extraction, snapshots, and an in-memory index.

The project passed through four on-disk generations:

| Generation | Version | Record format | Primary storage |
|---|---|---|---|
| 1 | `0.0.1-alpha.0` | `{<version>:<key>,<value>}` concatenated in one file | Flat append-only file |
| 2 | `0.0.1-alpha.1` | `{<key>:<value>}` in a versioned WAL | Brace-delimited WAL |
| 3 | `0.0.1-alpha.2` | `<tapari> <duna> <key> ;;<value>;; <unixnano> <fnv128a>` | Structured text WAL with checksum |
| 4 | `0.0.1-alpha.3` | `<seq> <tapari> <duna> <key> ;;<value>;; <unixnano> <fnv128a>` plus a checksummed JSON snapshot | Sequenced WAL + snapshot + in-memory index |

Three concerns recur throughout the history:

1. **Durability** — moving from unchecked appends to per-record `fsync`, then to atomic snapshot replacement (temp file, `fsync`, `rename`, directory sync).
2. **Integrity** — checksums that first covered only the key, then all fields, then the timestamp, then the sequence number.
3. **Concurrency** — a long, mistake-driven path from a misused global lock to a single WAL mutex and a documented lock order.

The project is also a candid record of refactoring hazards: a misplaced call that silently stopped writes; sixteen commits that did not compile, including a five-commit reader sequence; a nil-dereference in the history path; a snapshot that was written but never loaded; and checksum changes that invalidated existing records. Later commits correct each of these, and those corrections are the most instructive parts of the history.

Sixteen of the 141 commits (11%) could not be built in place. Seven broken windows account for them; the longest lasted 33 minutes and the longest sequence covered five consecutive commits. This is the clearest evidence that the project had no build gate: most errors are undefined names, unused variables, or stale call sites that a single `go build` would have caught immediately.

At `HEAD`, the program is functionally coherent: writes are durable in the WAL, memory is rebuilt from snapshot plus WAL replay, reads are served from memory, and history is regenerated from the WAL. The main engineering gap remains the absence of any automated test or build gate, which is why several defects survived only until the next manual run.

---

## 3. Release and Milestone Timeline

| Milestone | Commit(s) | Date and time (NPT) | Significance |
|---|---|---|---|
| First prototype | `3c18809` | 2026-10-12 00:46:49 | Flat-file append/read loop |
| Brace parser | `2b2480a` | 2026-10-12 01:09:25 | Records became addressable |
| History command | `f49e8e7` | 2026-10-12 02:55:26 | Per-key value history |
| JSON seeding | `bfcc211`, `0e34ea9` | 2026-10-12 04:27–04:57 | External data import |
| **Tag `v0.0.1-alpha.0`** | `351ca7e` (tag object `49a1ae5`) | Target 2026-10-12 05:00:57; tag created 2026-10-13 01:11:58 | End of flat-file prototype |
| WAL naming and object | `334dbf7`–`fd350a5` | 2026-10-13 02:39 – 2026-10-14 00:32 | DB/WAL separation, WAL struct |
| **Tag `v0.0.1-alpha.1`** | `fe3a9de` | 2026-10-14 02:35:24 | End of WAL-object/locking phase |
| Structured WAL v2 | `36fef1f`–`7e5325f` | 2026-10-14 04:32–05:48 | `tapari`/`duna`, checksum, unix nanos |
| Reader rewrite | `c53b857`–`ea07671` | 2026-10-14 18:13–18:28 | Explicit record parsing, shared scan |
| Hardening release | `6b21c1b` | 2026-10-16 23:47:47 | Validation, error returns, fsync |
| PR #1 merged | `a4b2805` | 2026-10-17 01:08:46 | `0.0.1-alpha.2` line consolidated |
| Checksum covers time | `063ee82`–`0aaeb6c` | 2026-10-18 00:25–00:27 | Timestamp integrity, single timestamp |
| In-memory DB + snapshots | `4770384`–`cc9b516` | 2026-10-19 17:47–23:23 | Memory index, JSON snapshot experiments |
| **Tag `v0.0.1-alpha.2`** | `cc9b516` | 2026-10-19 23:23:20 | Marker placed during snapshot work |
| Alpha.3 consolidation | `c7e7bc1` | 2026-10-19 23:54:58 | Sequenced WAL, snapshot load, replay |
| Agent instructions | `fa6e13a`, `d434d69` | 2026-10-19 23:55–23:59 | `AGENTS.md` and wording fix |

A note on tags: `v0.0.1-alpha.0` is an annotated tag created the day after its target commit. `v0.0.1-alpha.1` and `v0.0.1-alpha.2` are lightweight tags. The `alpha.2` tag points at `cc9b516` (*"loadign snapshort"*), not at the PR #1 merge, so tag names should be read as the author's bookmarks rather than as strict release boundaries.

---

## 4. Phase 0 — Flat-File Prototype (`v0.0.1-alpha.0`)

**Window:** 2026-10-12 00:46:49 → 2026-10-12 05:00:57 · **Commits:** #1–#56 · **Code size:** 39 lines at the start, 208 lines at the end.

### 4.1 Context and intent

The first commit describes the goal directly: *"initial code to write/append in to file and read all file."* The project began as a personal database (`AakkusPersonalDb.aakku`) with no server, no dependencies, and no abstraction beyond a single Go file. The initial design embedded the software version inside every record, which made the file self-describing at the cost of repetition.

The phase is dominated by rapid, small iterations: 56 commits in four hours and fourteen minutes. Many are formatting, renames, or committed data fixtures. The substantive work was record extraction, history grouping, and JSON import.

### 4.2 Significant changes

#### `3c18809` — 2026-10-12 00:46:49 — *Initial file-backed prototype*
- **Intent:** Store and retrieve key/value pairs by appending to a file.
- **Change:** `main.go` defines `Version = "0.0.1-alpha.0"` and `DbName = "AakkusPersonalDb.aakku"`. `write` opens the file with append/create and writes `{<version>:<key>,<value>}`. `read` dumps the raw file. Unknown commands have no handler.
- **Positive impact:** Proved the basic loop and introduced versioned records and a human-readable on-disk format.
- **Risks / regressions:** No argument-count validation before indexing `args[2]` and `args[3]`, so an incomplete `write` panics. No fsync. No read parser. The unused `Data` map and absent default branch show the code was scaffolding.
- **Defect status:** Latent panic, not fixed until the validation work in `6b21c1b` (#113).

#### `2b2480a` — 2026-10-12 01:09:25 — *Brace-based record extraction*
- **Intent:** *"extractRecords loops through the raw file data and pulls out each block wrapped in {}."*
- **Change:** A character scanner tracks `{`/`}` and returns record strings; `read` prints one record per line.
- **Positive impact:** Separated storage from presentation and made records individually addressable. This parser survived until the WAL v2 rewrite (#106).
- **Risks / regressions:** The format has no escaping. A `}` inside a value truncates the record; an unclosed block is silently discarded. The parser cannot distinguish corruption from a legitimate value.

#### `9fc0e80` + `00a6072` — 2026-10-12 01:21 — *Refactor miswired the write call*
- **Intent:** Extract file writing into `writeDb` (message: *"seperatign write to its function to de-claugher main func"*).
- **Change:** `writeDb` was introduced, but its call was placed inside the `read` branch. The `write` branch only printed a success message. `00a6072` moved the call 32 seconds later.
- **Positive impact:** Established the function-extraction pattern used for the rest of the project.
- **Risks / regressions:** For 32 seconds, `write` reported success without persisting, while `read` attempted to write using whatever arguments it had. This is the clearest early example of a refactor changing behaviour.
- **Defect status:** Confirmed regression, fixed by #13. See D-01.

#### `0ca97d6` → `53f076e` → `e7b3180` — 2026-10-12 01:44–02:20 — *Parser takes shape*
- **Intent:** Extract version, key, and value from each record.
- **Change:** The parser was first scaffolded in #14–#15 as `func extractKeyValue(records []string) FinalData` with no body. `0ca97d6` added a body but left `version`, `key`, and `value` unused and had no `return`, so it still could not compile. `53f076e` restored buildability with a stub; `e7b3180` finally accumulated a `map[string]string`.
- **Positive impact:** Converged on the `FinalData` map that made reads usable and enabled history grouping.
- **Risks / regressions:** Three consecutive commits (#14–#16) did not compile. Together with #18, this phase contains four non-buildable commits. The longest broken window lasted 33 minutes.
- **Defect status:** Confirmed build break, fixed by #17 (`53f076e`). See D-02.

#### `b016916` + `7af73d6` — 2026-10-12 02:28–02:37 — *Version filtering and brace stripping*
- **Intent:** Ignore records written by another software version, then simplify how the parser extracts a record.
- **Change:** First, records whose version field did not equal `SoftwareVersion` were skipped. Then `extractRecords` stopped returning the surrounding braces, and the version comparison was adjusted to trim a leading `{` before comparing.
- **Positive impact:** Established the idea that a file belongs to one version and that foreign records should not silently merge into the current view.
- **Risks / regressions:** The parser became tightly coupled to a literal syntax. The writer continued to emit braces while the parser stripped them, so correctness depended on both sides agreeing on that implicit convention. The version filter was later removed (#68) when the version moved into the filename.

#### `f49e8e7` + `195a49d` — 2026-10-12 02:55–03:05 — *History command*
- **Intent:** *"read history"* — show every value ever written for each key.
- **Change:** Added `history`/`h`, `buildKeyHistory`, and `displayHistory`. The first commit built the structure but did not display it; `195a49d` added the renderer.
- **Positive impact:** This became the project's defining feature: the append-only log is not just storage, it is an audit trail. The final alpha.3 design still derives history from the WAL.
- **Risks / regressions:** Go map iteration order made output nondeterministic. Duplicate writes of the same value are indistinguishable from distinct updates. Deterministic sorting did not arrive until `6b21c1b` (#113).

#### `fae00e1` → `210e122` → `2f46d19` — 2026-10-12 03:31–03:57 — *History persistence format churn*
- **Intent:** Save history to `history.aakku` and reduce its size.
- **Change:** The format changed three times in 26 minutes: arrow-separated text, then `version{{key:v1,v2,}}`, then `version:{key:v1,v2,}` with the outer closing brace removed. Fixtures were committed alongside each change.
- **Positive impact:** History survived process exit and became compact enough to inspect manually.
- **Risks / regressions:** Intermediate committed fixtures do not all conform to the final convention: `history.aakku` reflects the no-closing-brace format, while `History0.0.1-alpha.1.aakku` (committed in the next phase) starts directly with per-key braces and has no version prefix. Because these are generated fixtures rather than inputs, the inconsistency was cosmetic, but it demonstrates the cost of committing a format before it stabilizes. The version prefix was removed entirely in #63.

#### `7d7e0ca` → `0e34ea9` — 2026-10-12 04:01–04:57 — *JSON seeding and type flattening*
- **Intent:** Import an arbitrary JSON document into the database by adding a parser for the JSON file.
- **Change:** `data.json` was added (initially invalid JSON, fixed in `c2ab778`), a `parse`/`p` command was added, and each key/value pair was appended to the WAL. Importing non-string values first used `value.(string)`, which panics on numbers, maps, and null; `0e34ea9` replaced it with `fmt.Sprintf("%v", value)`.
- **Positive impact:** Demonstrated bulk ingestion from external data and forced the code to accept heterogeneous JSON.
- **Risks / regressions:** The final approach flattens every value to a string with Go's default formatting. Maps become `map[city:New York zipCode:10001]`, slices become `[Go Python JavaScript]`, and null becomes `<nil>`. Types are not preserved and the representation is lossy and locale-like rather than machine-readable.
- **Defect status:** Type-assertion panic confirmed and fixed (D-05); lossy typing remains an open design limitation (O-09).

#### `351ca7e` — 2026-10-12 05:00:57 — *`v0.0.1-alpha.0` baseline*
- **Intent (tag message):** *"This is its 1st release, i am goign to make some bracking changes so thought its good to tag."*
- **Positive impact:** Created a reproducible baseline before the storage format was reworked.
- **Risks / regressions:** The tag was created on 2026-10-13 at 01:11:58, about 20 hours after its target commit and before the next code phase began at 02:39, so it is a retrospective marker rather than a contemporaneous release event.

### 4.3 Supporting commits (grouped)

The remaining commits in this phase are incremental polish and fixtures: `fcbdf32`–`70e5f82` (#2–#6: type/const scaffolding, output formatting, fixture rows), `8ce03c2`–`8320e73` (#8–#11: wiring the parser into `read`, comments, example output), `af291d4`–`0ca97d6` (#14–#16: parser scaffolding, named type, `SoftwareVersion` rename; all three non-buildable), `df6bbea`–`a520e84` (#18–#25: debugging cleanup, renames, map-based `sanitizeRecords`; #18 also non-buildable), `9368c82`–`98ccce7` (#27–#32: `readAllDb`, display function, cleanups), `51456bb`–`1658ae9` (#33–#34: print error handling), `38f44ab`–`bc841d7` (#36–#42: history persistence polish), and `76a1b34`–`8a64760` (#48–#55: display, JSON import wiring, fixtures). Appendix A lists every commit with its original message.

### 4.4 Phase assessment

**What went well**

- A complete write → read → history loop existed within a few hours.
- The append-only log plus derived history proved to be a durable architectural idea.
- JSON import exposed the value-typing problem early.

**What went wrong**

- No build or test gate allowed four non-compiling commits and a miswired branch to enter history.
- The on-disk format changed faster than fixtures could be kept consistent.
- Reads were nondeterministic because map iteration was not sorted.
- Values were never typed; every JSON type was flattened to a string.

---

## 5. Phase 1 — WAL Identity and Locking (`v0.0.1-alpha.1`)

**Window:** 2026-10-13 02:39:00 → 2026-10-14 02:35:24 · **Commits:** #57–#91 · **Code size:** 268 lines.

### 5.1 Context and intent

The author stopped calling the storage file a "database" and began calling it a WAL. This phase introduced the `HoneyberriWAL` object, moved functions onto it as methods, added locking, and renamed the domain concepts. The version was removed from each record because the filename now carried it. The phase ends with the first committed binary and the `alpha.1` tag.

### 5.2 Significant changes

#### `334dbf7`–`4701fe2` — 2026-10-13 02:39–02:42 — *Rename to WAL and HoneyberriDB*
- **Intent:** *"thats not DB thats wal"* and *"official DB name is honey Berry DB."*
- **Change:** `DbName` became `WALname`; `DBname = "HoneyberriDB"` was introduced; all read/write call sites were updated.
- **Positive impact:** Established a distinction that becomes real in alpha.3: the database is the in-memory layout, while the WAL is the persistent log.
- **Risks / regressions:** Naming churn consumed four commits. The first three (#57–#59) did not compile because some call sites still referenced `DbName`; #60 completed the rename. No behavioural change once buildable.
- **Defect status:** Confirmed build break, fixed by #60. See D-19.

#### `6f8b452`–`1aab938` — 2026-10-13 02:47–02:51 — *Version moves into the filename*
- **Intent:** *"Since WAL will have version on it file name so thers no point on printing version on each entries."*
- **Change:** Bumped to `0.0.1-alpha.1`; records became `{<key>:<value>}`; history filenames became `History<version>.aakku`; the version prefix was removed from history content.
- **Positive impact:** Reduced repetition, made file provenance explicit, and established versioned filenames as the project's compatibility boundary.
- **Risks / regressions:** Readers of the previous format cannot read the new one and vice versa. There is no migration or dual-read path. This trade-off is revisited in #114 and #139 and remains the project's policy.

#### `1c25f3e` + `030a3e8` — 2026-10-13 03:46 / 22:46 — *First-colon split attempt and repair*
- **Intent:** Split key and value at the first `:` only, because values can contain colons (for example, `map[city:...]`).
- **Change:** The first attempt used a `done` flag inside a `FieldsFunc` callback. Because the callback runs once per rune, `done` was reinitialized every time and the function still split at every colon; there was also no guard for records without a colon. `030a3e8` replaced the logic with `strings.Cut` and `TrimSpace`.
- **Positive impact:** `strings.Cut` is correct, simple, and readable; the same pattern was reused for history.
- **Risks / regressions:** For roughly 19 hours the intended behaviour did not exist, and a malformed record could panic on `words[1]`.
- **Defect status:** Confirmed, fixed by #73. See D-06.

#### `5d225a6`–`b6c6b6f` — 2026-10-13 23:12 → 2026-10-14 00:41 — *The WAL locking saga*
- **Intent:** Make concurrent WAL writes safe.
- **Change:** `5d225a6` added a global `sync.RWMutex`, locked it, opened the file, then **unlocked before writing** — so the lock protected only file opening. `fd350a5` introduced the `HoneyberriWAL` struct with a per-instance mutex and an `OpenWAL` constructor. `fd0de23` converted functions to methods and added `wal.mu.Unlock()` at the end of `writeWAL` without a matching `Lock`, which guarantees a runtime panic. `b6c6b6f` finally wrapped the shared file handle write in `wal.mu.Lock()`/`Unlock()`.
- **Positive impact:** The final shape — one open file handle and one mutex per WAL — is still the design at `HEAD`. `b6c6b6f` is the first correct write lock in the project.
- **Risks / regressions:** Three flawed iterations, one guaranteed crash, and an ineffective lock that could have allowed interleaved writes. The episode is the strongest argument in the history for small, reviewed concurrency changes.
- **Defect status:** Confirmed regressions D-07 and D-08, both fixed by #81.

#### `d50a86d`–`bfa974c` — 2026-10-14 01:17–01:19 — *Reading through the WAL handle*
- **Intent:** Read through the open file handle rather than reopening with `os.ReadFile`.
- **Change:** `readAllWAL` and `readAllHistory` were moved onto the WAL, using `Seek(0, io.SeekStart)`, `Stat`, and a single `Read`. The first version locked the mutex and never unlocked; `bfa974c` added `defer wal.mu.Unlock()` and empty-file handling.
- **Positive impact:** Explicit offset management, a single file handle, and the reason the WAL uses `sync.Mutex` rather than `RWMutex`: scanning moves the shared file offset.
- **Risks / regressions:** The initial version could deadlock any later locked operation. The single `Read` is not guaranteed to fill the buffer for all file types; it was later replaced by a buffered scanner (#105). The commit body already anticipated this: *"this func seeks to 0 and read all log file at once, i will fix that to fixed size chunk in next version *alpha.2."*
- **Defect status:** Deadlock risk confirmed by inspection and fixed by #85 (D-09).

#### `98bf4f6` — 2026-10-14 02:05 — *First committed binary*
- **Intent:** Ship a runnable binary alongside the data/history fixtures; a neighboring commit jokes about a *"bigger inary."*
- **Change:** Added the compiled `main` executable to Git; it was rebuilt twice (`df1b466`, `1561c6a`).
- **Positive impact:** Made a runnable artifact available without a Go toolchain.
- **Risks / regressions:** The binary is macOS/arm64-specific, roughly 3.7 MB, and was last rebuilt on 2026-10-14, long before alpha.3. Running `./main` today executes pre-snapshot code against a different format. It remains in the repository at `HEAD`.
- **Open risk:** O-08.

#### `fe3a9de` — 2026-10-14 02:35:24 — *`v0.0.1-alpha.1`*
- **Change:** Removed the now-unused global `WriteWalLock`. The commit body notes: *"i wonder why compiler didnt scolded me wired"* — a reference to the fact that package-level unused variables are legal in Go.
- **Positive impact:** Closed the phase with the per-WAL lock as the only lock.
- **Risks / regressions:** The tag is lightweight, so it carries no release message beyond the commit itself.

### 5.3 Supporting commits (grouped)

The rest of the phase is naming and formatting: `db16fe0`–`d8373d2` (#58–#59: official name, casing), `8bc6503`–`b1c7ef2` (#64–#69: split-on-colon cleanup, index shifts after version removal, WAL fixture), `a2193da`–`3ef0fac` (#70–#72: alpha.1 WAL/history fixtures, JSON rows), `daa3bc3`–`3579fc6` (#82–#83: formatting), `1470c69`–`df95a4e` (#86–#87: history read via the WAL handle, split read/print), and `a535928`–`df1b466` (#89–#90: history reuses the shared reader, fixture and binary update). Appendix A has the complete list.

### 5.4 Phase assessment

**What went well**

- The WAL abstraction became real: one handle, one lock, methods instead of free functions.
- Version-in-filename removed per-record repetition and gave the project a workable compatibility boundary.
- `strings.Cut` replaced fragile hand-rolled splitting.

**What went wrong**

- The lock was wrong in three different ways before it was right.
- The read path initially leaked a lock.
- A three-commit rename window (#57–#59) did not compile.
- The first binary was committed and never brought up to date.
- Intermediate formats and fixtures changed without a build gate.

---

## 6. Phase 2 — WAL v2: Structured Records and Checksums (`0.0.1-alpha.2`)

**Window:** 2026-10-14 04:26:27 → 2026-10-14 18:28:00 · **Commits:** #92–#110.

### 6.1 Context and intent

The brace format could not represent spaces, delimiters, timestamps, or integrity data safely. This phase replaced it with a line-oriented record containing four header fields, a delimited value, a unix-nanosecond timestamp, and an FNV-128a checksum. It also introduced the domain terms `tapari` and `duna` and a reader that replays records into a state map.

The phase contains six commits that did not compile: `36fef1f` (missing imports) and the five-commit reader sequence `c53b857`–`ad7b809`. The first was fixed in 19 seconds; the reader sequence took 14 minutes to become buildable again. The phase also contains the first checksum, which initially covered only the key and was then expanded to all fields.

### 6.2 Significant changes

#### `a593a41` + `7efee1e` — 2026-10-14 04:26 — *Alpha.2 and usage text*
- **Change:** Version bumped to `0.0.1-alpha.2`; the top-level error message was updated to list `write, read, history, parc`.
- **Positive impact:** Clean version boundary for the new format.
- **Risks / regressions:** The usage text still described a pre-validation CLI and was replaced again in #113.

#### `36fef1f` + `fc6305b` — 2026-10-14 04:32 — *Structured record v1, and a broken build*
- **Intent:** *"thats new logging pattern"* — move from brace records to `tapari duna key value hash`. The commit body defines the model: a **duna** is a collection of key/value pairs (like a table or collection), and a **tapari** is a collection of dunas (like a namespace or database).
- **Change:** `writeWAL` took four positional fields, hashed the key with FNV-128a, and wrote `<tapari> <duna> <key> <value> <hash>`. The commit used `hash/fnv` and `io` without importing them; `fc6305b` added the imports 19 seconds later.
- **Positive impact:** Introduced the `(tapari, duna)` grouping that survives to `HEAD`, and the first integrity hash.
- **Risks / regressions:** The commit was not buildable. The delimiters were spaces, so a value containing spaces could not be represented (the CLI only took one value argument). The hash covered only the key, so value corruption was undetectable. The commit body also says that creating a second duna with the same name should throw an error; the implementation instead reuses the existing map, so that rule remains documentation rather than enforced behaviour.
- **Defect status:** Confirmed build break D-10; checksum weakness D-11, fixed in #101.

#### `a3921e6`–`f2e1bfd` — 2026-10-14 05:05–05:36 — *Delimiters, time, and full-field checksum*
- **Intent:** Make values with spaces parseable and make the checksum cover the whole record.
- **Change:** Added `TrimSpace`, wrapped values in `;;...;;`, wrote `time.Now().UTC()` as a formatted timestamp, encoded the hash as hex, and changed the checksum payload to `;;<tapari>;;<duna>;;<key>;;<value>;;`.
- **Positive impact:** Hex made the digest readable; full-field hashing meant a changed value or key would be rejected on read; `;;` gave values a boundary that spaces could not break.
- **Risks / regressions:** The formatted timestamp contains spaces (`2026-10-13 23:26:51.685221 +0000 UTC`), complicating line parsing. The checksum still excluded the timestamp.

#### `7e5325f` + `6d95775` — 2026-10-14 05:48 — *Unix-nanosecond timestamps*
- **Intent:** Use a stable, machine-readable time representation.
- **Change:** Replaced the formatted time with `time.Now().UnixNano()` (decimal). Fixture rows were added in the new format.
- **Positive impact:** No spaces, no timezone ambiguity, sortable, and suitable for exact checksum coverage. This is the timestamp format used at `HEAD`.
- **Risks / regressions:** The committed alpha.2 WAL now contains a mixture of formatted and numeric timestamps, forcing the reader to support both. That compatibility code was later removed deliberately (#114).

#### `c53b857`–`ad7b809` — 2026-10-14 18:13–18:27 — *Reader rewrite and a five-commit build break*
- **Intent:** *"new way to read new WAL style."*
- **Change:** `c53b857` introduced `Record`, a nested `state` map, `replay`, `parseLine`, `parseTimestamp` (accepting both numeric and formatted time), sorted display, and a skipped-line counter. The old brace parser was marked deprecated. The commit omitted `sort` and `strconv` imports. `201c806` removed the brace parser but still missed the imports and left a call to `extractRecords`; `1561c6a` removed the old byte-reader but still called `readAllWAL` and `extractRecords`; `5907b70` added the imports but left both calls; `ad7b809` renamed fields throughout the code but left `DisplayWAL` referencing `rec.DB`/`rec.Bucket`. `ea07671` (#110) was the first buildable commit in the sequence.
- **Positive impact:** Replaced ad-hoc string extraction with an explicit parser and made "last write wins" the read semantics. The skipped-line counter gave corruption visibility without aborting the whole read. The final shape is the reader used for the rest of the project.
- **Risks / regressions:** Five consecutive commits were not buildable for ~14 minutes. Supporting two timestamp formats added branching and ambiguity that later had to be removed. The sequence also removed the old reader before its replacement compiled, so there was no working fallback in the tree.
- **Defect status:** Confirmed build break D-12, fixed by #110 (`ea07671`).

#### `201c806` + `1561c6a` + `ad7b809` — 2026-10-14 18:21–18:27 — *Removal and domain naming*
- **Change:** Deleted the brace parser and the old byte-reader; renamed `DB`/`Bucket` to `Tapari`/`Duna`.
- **Positive impact:** Removed dead code and aligned identifiers with the project's vocabulary. The rename makes the data model clearer than generic database terms.
- **Risks / regressions:** These three commits are part of the five-commit non-buildable sequence above: removed functions and a renamed field were still referenced. `1561c6a` also rebuilt the committed binary, which was already drifting from the source.
- **Defect status:** Covered by D-12.

#### `ea07671` — 2026-10-14 18:28 — *Shared scanner and WAL-derived history*
- **Intent:** *"Re-implementation of ReadWal and making History through wal."*
- **Change:** Added `scanWAL(fn)` so reads and history share one pass; `ReadWAL` builds state, history builds value lists, and history renders to stdout and a file. Removed `displayDb` and the previous replay helper. This is the first buildable commit after the reader sequence.
- **Positive impact:** One parser and one traversal for both views; history now grouped by `tapari`/`duna`; output sorted.
- **Risks / regressions:** History renders twice (stdout, then file) until #113 replaces it with `io.MultiWriter`. Both reads and history scan the entire WAL, an O(n) cost that persists at `HEAD`. `readAllHistory` still used `log.Fatalln` rather than returning errors.

### 6.3 Supporting commits (grouped)

`7d73c13`, `bd7c981`, and `6d95775` are WAL fixtures showing the format transition. `5907b70` adds the missing imports but does not restore buildability; the sequence remains broken until #110. `201c806`–`ad7b809` cover the cleanup and rename sequence. Appendix A lists each commit with its original message.

### 6.4 Phase assessment

**What went well**

- The record format became explicit and line-oriented, with a real parser and a corruption counter.
- The checksum evolved from a key-only digest to a full-record digest.
- Unix nanos removed timezone and whitespace ambiguity.
- `tapari`/`duna` gave the data model a stable vocabulary.

**What went wrong**

- Six commits were not buildable: one import error and a five-commit reader rewrite.
- Compatibility for the transitional timestamp format was added and then removed, creating a deliberate data-visibility break.
- History and read still scanned the whole WAL; no in-memory state existed yet.

---

## 7. Phase 3 — Hardening and PR #1 (`0.0.1-alpha.2` release line)

**Window:** 2026-10-16 23:15:18 → 2026-10-17 01:08:46 · **Commits:** #111–#123.

### 7.1 Context and intent

After two days of format work, the project paused to harden the code before merging the alpha.2 branch. The key commit is `6b21c1b` (*"FIx/Changes:"*), a large refactor that added validation, error returns, `fsync`, a shared checksum function, and a buffered WAL scanner. The phase ends with the GitHub merge of PR #1.

### 7.2 Significant changes

#### `e9f9674` + `14248d6` — 2026-10-16 23:15–23:18 — *Fixture and generated history*
- **Change:** Added a test WAL row and a generated `History0.0.1-alpha.2.aakku`; the commit message notes an *"Auto commit + push script ran from terminal"*.
- **Positive impact:** Provided realistic data for the new reader and a snapshot of expected history output.
- **Risks / regressions:** Generated files were committed as fixtures with no indication of how to regenerate them. The history filename changed later (#120), leaving a duplicate artifact.

#### `6b21c1b` — 2026-10-16 23:47:47 — *Hardening release*
- **Intent:** Consolidate the code after rapid format churn; the message is simply *"FIx/Changes:"*.
- **Change:**
  - `run` now returns errors and validates argument counts; `write` accepts multi-word values by joining `args[5:]` with spaces.
  - `newRecord` validates `tapari`, `duna`, and `key` (non-empty, no whitespace, no `;;`) and rejects line breaks in values.
  - A single `checksum` function is shared by writer and reader; at this commit it covers `tapari`, `duna`, `key`, and `value` (the timestamp is added later in #124); `appendRecord` writes once and calls `fsync`.
  - `scanWAL` uses `bufio.Scanner` with a 16 MB maximum line, skipping and counting invalid lines.
  - `ReadWAL` and `ReadHistory` return errors instead of calling `log.Fatalln`; output is sorted and buffered.
  - History uses `io.MultiWriter(os.Stdout, file)` so it renders once.
  - Removed the `parse` command and the unused JSON import path.
- **Positive impact:** The largest quality jump in the project. It fixed the malformed error formatting, introduced real validation, made WAL writes durable, made output deterministic, and aligned writer/reader hashing. Most of these choices survive at `HEAD`.
- **Risks / regressions:** Behaviour changed substantially in one commit (multi-word values, removed `parse`, stricter validation), with no tests. Formatting/scanning edge cases (delimiter collisions, 16 MB limit) are now explicit but untested.
- **Defect status:** Fixed several latent validation and error-handling issues; no single register entry corresponds to the whole refactor. It did not yet cover the timestamp, which becomes relevant in #124.

#### `a02373f` — 2026-10-17 00:04 — *Backward-compatibility removal*
- **Intent (paraphrased from the commit message):** Remove support for the old timestamp format and read only UnixNano; the message also lists *"removed backcompatability"* and *"Reduced complex compatable logic"* as goals.
- **Change:** Deleted `timeLayout`, `parseTimestamp`, and the multi-field tail parsing. A record now requires exactly two tail fields: numeric nanos and hash.
- **Positive impact:** One unambiguous timestamp format; simpler parser; checksum verification is exact.
- **Risks / regressions:** Every record written with the formatted timestamp is now rejected as corrupt. In the committed `HoneyberriDB0.0.1-alpha.2.aakku`, the earliest eleven records use that format, so a reader at this commit would skip them and report corruption. The history file committed at `1f5b149` confirms the loss: it contains `hobby` (which also appears in numeric-timestamp rows) but omits `HumainDb AdarashaBucket Name` (Adarasha Gaihre), which exists only in the formatted records. The alpha.3 filename change later isolates this historical data, but there is no migration.
- **Compatibility break:** C-02 in Section 10.2.

#### `590d634` — 2026-10-17 00:42 — *Leading newline*
- **Intent:** The commit is titled *"I have a BIG BRAIN"* and its body explains the durability goal: *"Even if the app crasses in middle of writing, two records wont be corrupted, oly one will be corrupted, the next comming wouldn't be affected."*
- **Change:** Each record is now prefixed with `\n` instead of suffixed.
- **Positive impact:** A partial write can damage only the record being written. The next record starts with its own newline, so it cannot be glued to a truncated predecessor. Appending also works when the previous file does not end with a newline.
- **Risks / regressions:** The WAL begins with an empty line and does not end with a newline. External tools that expect a trailing newline see a slightly unusual file; the parser handles it.

#### `1a3cbf7`, `5c37017`, `204fd99`, `933bba9` — 2026-10-17 00:46–00:47 — *Independent version axes*
- **Intent:** Give the WAL format its own version because it cannot share the software/DB version (paraphrased from the commit message).
- **Change:** Added `WALversion`; `WALname` uses it; `HistoryName` became `History.<DB><WALversion>`.
- **Positive impact:** The WAL format and the software can now evolve independently. History follows the WAL version, which is correct because it is derived from WAL records. This design survives at `HEAD` (snapshot follows `SoftwareVersion`, WAL/history follow `WALversion`).
- **Risks / regressions:** More constants to keep synchronized; a mistake in one constant silently points at a different dataset.

#### `8ad29a0` + `1f5b149` — 2026-10-17 00:53–00:54 — *Fixtures and history rename*
- **Change:** Added formatted WAL rows and the new `History.HoneyberriDB0.0.1-alpha.2.aakku`.
- **Positive impact:** Verified the new naming and grouping in a committed artifact.
- **Risks / regressions:** The new history file omits values that exist only in the formatted-timestamp records. For example, `HumainDb AdarashaBucket Name` (Adarasha Gaihre) appears in the first eleven WAL records but not in the history committed here; numeric-timestamp values such as `hobby` and `HumainDb aakkuTapari` are present. The file reflects what the post-#114 reader could see, not the full WAL.

#### `a4b2805` — 2026-10-17 01:08:46 — *Merge PR #1*
- **Change:** GitHub merge of branch `aakku106/0.0.1-alpha.2` into `main`; second parent `1f5b149`; body *"0.0.1 alpha.2."*
- **Positive impact:** Consolidated the alpha.2 line and gave the project a clear integration point.
- **Risks / regressions:** The merge itself contains no code changes, and no CI ran because none is configured. The `alpha.2` tag was not placed here; it appears two days later at #138.

### 7.3 Supporting commits (grouped)

`dab5f96` (#115) is a data row. `1a3cbf7`–`933bba9` (#116–#120) are the version-constant/naming sequence. `8ad29a0` (#121) and `1f5b149` (#122) are fixtures. Appendix A has the full list.

### 7.4 Phase assessment

**What went well**

- Validation and durable writes turned the prototype into a defensible storage engine.
- One checksum function removed writer/reader drift.
- Independent WAL/software versions clarified file ownership.

**What went wrong**

- A deliberate compatibility break made older records unreadable without migration.
- Release tagging lagged the actual integration point.
- Generated fixtures were committed without regeneration instructions.

---

## 8. Phase 4 — In-Memory Database and Snapshots

**Window:** 2026-10-18 00:25:16 → 2026-10-19 23:23:20 · **Commits:** #124–#138 · **Includes tag `v0.0.1-alpha.2`.**

### 8.1 Context and intent

Until this phase, every read replayed the WAL and every write appended to it. The author began building an in-memory database to make reads O(1) and a snapshot to avoid replaying the full WAL at startup. The phase is experimental: the snapshot is written before it is ever loaded, the history command briefly crashes, and the tag is applied before the feature is complete.

### 8.2 Significant changes

#### `063ee82` + `11db769` + `0aaeb6c` — 2026-10-18 00:25–00:27 — *Timestamp enters the checksum*
- **Intent:** *"checksum covers every field of the record except the hash itself."*
- **Change:** `checksum` gained a `nanos` parameter. `063ee82` changed the signature but left both call sites at four arguments, so it did not compile. `11db769` fixed the writer call and captured `now` once for both the stored timestamp and the digest, but left the reader call broken. `0aaeb6c` fixed the reader and is the first buildable commit.
- **Positive impact:** Timestamp corruption is now detected, and a single `now` value feeds both the stored time and the digest, so they cannot diverge. The final design survives at `HEAD` (extended with `seq` in #139).
- **Risks / regressions:** The two-minute writer/reader mismatch is only part of the impact. Because the checksum formula changed, **every record written before #124** — whose hash excluded nanos — also fails verification under the new reader. The fixture refresh in `814c401` shows the result: the regenerated history drops `CatDb CatBucket`, `HumainDb hobby`, and `HumainDb aakkuTapari`. There is no migration, dual-hash check, or WAL version bump.
- **Defect status:** D-13 (integrity regression) and D-20 (build break), fixed by #126.

#### `814c401` — 2026-10-18 00:31 — *Fixture refresh and the checksum cull*
- **Change:** Regenerated the WAL and history fixtures after the checksum change, appending fresh rows for some keys.
- **Positive impact:** Gave the new checksum a committed sample and restored several keys with valid hashes.
- **Risks / regressions:** The regenerated history shrinks from the `1f5b149` version: `CatDb CatBucket`, `HumainDb hobby`, and `HumainDb aakkuTapari` disappear because their records were written with the older checksum (without nanos) and now fail verification. Only the keys that were re-appended survive. This is the concrete data-visibility cost of extending checksum coverage without a migration or dual-hash read path.
- **Compatibility break:** C-06 in Section 10.2.

#### `4770384` + `88c55f5` — 2026-10-19 17:47 — *In-memory DB skeleton*
- **Intent:** *"creatign ans sendign db refrence"* and *"implementation of in memory DB."*
- **Change:** Added `HoneyberriDB` with `DbLayout map[string]map[string]map[string]string` and an `RWMutex`; `CreateDB`; `updateDb` creates nested maps and stores the value. The write path appends to the WAL, then calls `updateDb`. The first commit (#128) did not compile because it referenced `CreateDB`, which was introduced by #129 17 seconds later.
- **Positive impact:** First persistent-in-process representation of current state; writes no longer need a read-back to be visible.
- **Risks / regressions:** The DB starts empty on every process; it is not rebuilt from the WAL. It stores no timestamp, duplicating layout logic. `fmt.Errorf("Unable to update DB", err)` has no format directive, producing a misleading error and a `go vet` warning.
- **Defect status:** D-14 (error formatting) and D-21 (build break); D-14 fixed in #133, D-21 in #129.

#### `4c16777` + `44ac017` — 2026-10-19 18:14 — *JSON snapshot*
- **Intent:** Persist the in-memory layout to disk (*"snapshottingdb"*).
- **Change:** `DBsnapShot` encodes `DbLayout` as JSON through a buffered writer. `main` defers the call. `44ac017` turns a free `CloseDb` function into a method on `HoneyberriDB`.
- **Positive impact:** Introduced the snapshot concept and an inspectable JSON representation.
- **Risks / regressions:** The snapshot is taken on every command, even reads. It is **not loaded at startup**, so a write-only run overwrites a fuller snapshot with only the current session's state; a read-only run at this commit writes an empty snapshot because reads still scan the WAL without touching the DB. Errors from the deferred snapshot are discarded. The file is not written atomically and has no checksum. The filename is inconsistently `SnapShot.aakku`/`snapShot.aakku`.
- **Defect status:** D-17, fixed by #139.

#### `6d9796b` — 2026-10-19 18:20 — *Error formatting and ordering*
- **Change:** Fixed the missing format verb (`"Unable to update DB:\t%s"`), moved the success print after the memory update.
- **Positive impact:** A failed update no longer prints success first; the error includes the cause.
- **Risks / regressions:** Still uses `%s` rather than `%w`, so the error cannot be unwrapped.

#### `211172e` + `aff3238` — 2026-10-19 18:20–18:56 — *Snapshot fixtures*
- **Change:** Added `snapShot.aakku` and WAL rows; the snapshot later expanded to the full layout.
- **Positive impact:** Concrete examples of the JSON layout.
- **Risks / regressions:** The snapshots are plain JSON with no sequence number or checksum, so they are not the format used by the final loader.

#### `b7e785c` — 2026-10-19 18:56 — *Loading the WAL into memory during read*
- **Intent:** *"snapshottig while reading all wal."*
- **Change:** `CreateDB` became `OpenDb`; `scanWAL` gained a `db` parameter and applies every record to memory; `ReadWAL` passes the DB; history passes `nil`.
- **Positive impact:** A `read` now populates the in-memory DB in the same pass that renders it.
- **Risks / regressions:** History calls `scanWAL(..., nil)`, and `scanWAL` unconditionally calls `db.updateDb`, so `history` panics on the first valid record. `scanWAL` holds `wal.mu` while `updateDb` takes `db.mu`, creating the reverse of the order later adopted in alpha.3. The snapshot is still never loaded. `ReadWAL` also builds a second state map alongside the DB, duplicating work.
- **Defect status:** D-15 (nil dereference), D-16 (lock order), D-17 (snapshot not loaded), all fixed by #139.

#### `303dd4e` — 2026-10-19 22:59 — *Dead `load` command*
- **Change:** Added `case "load", "l":` with a comment *"this shall read all wall, and build DB in memory"* and `return nil`.
- **Positive impact:** None; it documented an intention.
- **Risks / regressions:** The command silently succeeds without loading anything, which is worse than not offering it. Removed in #139.
- **Defect status:** D-18.

#### `cc9b516` — 2026-10-19 23:23 — *`LoadSnapshot` added but never used; tag alpha.2*
- **Intent:** *"loadign snapshort."*
- **Change:** Added `LoadSnapshot`, which opens the JSON file, decodes into `DbLayout`, and treats a missing file as a fresh DB. It is not called anywhere. The function also has no checksum or sequence validation.
- **Positive impact:** Reusable loading code and the seed of the recovery path implemented in #139.
- **Risks / regressions:** The feature appears complete but is inactive; snapshots are still only written. The lightweight `v0.0.1-alpha.2` tag was placed here, after snapshot work began and before alpha.3, so it is not a clean release marker.
- **Defect status:** D-17.

### 8.3 Supporting commits (grouped)

`814c401`, `d1095ed`, `211172e`, and `aff3238` are data/fixture updates. `44ac017` is a small method conversion. Appendix A contains every commit.

### 8.4 Phase assessment

**What went well**

- The in-memory representation and snapshot concept were introduced.
- The timestamp checksum defect was caught and fixed within minutes.
- The failed snapshot experiments produced the design requirements for alpha.3: load at startup, sequence numbers, atomic writes, checksums, and error propagation.

**What went wrong**

- Two build windows (#124–#125 and #128) entered history; the checksum one also changed the on-disk formula.
- Extending the checksum to the timestamp invalidated every earlier record, and the regenerated history silently lost groups.
- The snapshot was written but not loaded, so it could silently replace a fuller snapshot with partial state.
- History crashed because a scan side effect was passed a nil DB.
- The lock order inverted.
- A command (`load`) was advertised but did nothing.
- The alpha.2 tag was applied to an incomplete transition.

---

## 9. Phase 5 — Alpha.3 Consolidation and Documentation

**Window:** 2026-10-19 23:54:58 → 2026-10-19 23:59:51 · **Commits:** #139–#141.

### 9.1 `c7e7bc1` — 2026-10-19 23:54:58 — *Sequenced WAL, snapshot loading, and replay*
- **Intent (commit message):** *"new wal+software version with sequence number and proper in memory db load,read,write and snapshot."*
- **Change:**
  - Bumped `SoftwareVersion` and `WALversion` to `0.0.1-alpha.3`; added `SnapshotName`.
  - Added `Record.Seq`; the checksum now includes the sequence number; encode/parse use a four-field header.
  - `OpenDb` loads the snapshot, verifies its checksum, then replays only WAL records with `seq > snapSeq`. A bad snapshot is non-fatal: it warns and replays the full WAL.
  - `Set` assigns `lastSeq+1`, appends and fsyncs to the WAL, applies to memory, and advances `lastSeq` — all under `db.mu`.
  - `PrintAll` serves reads from memory; history still scans the WAL.
  - The snapshot is now `{seq, layout}`, encoded as JSON plus an FNV-128a checksum, written to a temp file, fsynced, renamed, and followed by a best-effort directory sync. `snapMu` serializes writers; a snapshot is skipped when the sequence has not advanced.
  - `realMain` propagates errors, always closes the WAL, and checks the snapshot error.
  - Removed the dead `ReadWAL`, `load` command, and empty `OpenDb`.
- **Positive impact:** This is the first version whose startup state is actually correct. It fixes the snapshot-load, nil-dereference, lock-order, dead-command, ignored-error, and partial-snapshot defects of the previous phase. Reads no longer rescan the WAL, replay is bounded by the snapshot sequence, and snapshots are atomic and integrity-checked.
- **Risks / regressions:**
  - The engine is single-process only; there is no file lock, so two CLI runs can assign the same sequence number and interleave writes or snapshot temp files.
  - The WAL is never compacted or truncated. History remains an O(total writes) scan and disk usage grows forever.
  - The snapshot contains only the latest value per key; history exists only in the WAL.
  - FNV-128a detects accidental corruption but is not cryptographic; records can be forged by anyone who can write the file.
  - Sequence numbers are trusted, not validated for monotonicity or gaps.
  - There are still no tests.
- **Defect status:** Resolves D-15 through D-18. The risks above are open at `HEAD` and are listed in Section 10.3.

### 9.2 `fa6e13a` — 2026-10-19 23:55:53 — *`AGENTS.md`*
- **Intent (commit message):** *"found out opencode alows some free tear aimodels just testing them to write some doc aand init file."*
- **Change:** Added agent-facing instructions covering the build command, CLI, temp-directory testing, versioned filenames, format invariants, lock order, and the stale binary.
- **Positive impact:** Encodes operational knowledge that was previously implicit, especially the fact that running the tool in the repository root pollutes the working directory and that only alpha.3 files are live.
- **Risks / regressions:** It is not user documentation and has no README counterpart. Like any instruction file, it can drift from the code; it must be updated when formats or versions change.

### 9.3 `d434d69` — 2026-10-19 23:59:51 — *Data-file wording correction*
- **Change:** Corrected the claim that running commands in the repository root modifies tracked data files; the current build writes untracked alpha.3 filenames, and would only overwrite a tracked legacy file if the version constants were reset to one of those names.
- **Positive impact:** More accurate guidance for future agents.
- **Risks / regressions:** None.

### 9.4 Phase assessment

**What went well**

- The storage model finally became internally consistent: WAL is the durable source of truth, snapshot accelerates startup, memory serves reads, history is a derived view.
- The previous phase's defects were all addressed in one coherent refactor.

**What went wrong**

- No tests were added alongside the most complex change in the project.
- Single-process assumptions, unbounded WAL growth, and non-cryptographic checksums remain undocumented in user-facing material (though `AGENTS.md` covers some operational details).

---

## 10. Defect and Risk Register

### 10.1 Confirmed defects

| ID | Introduced | Fixed | Defect | Consequence |
|---|---|---|---|---|
| D-01 | `9fc0e80` #12 | `00a6072` #13 | `writeDb` called from the `read` branch | `write` reported success without persisting; `read` attempted to write |
| D-02 | `af291d4`–`0ca97d6` #14–#16 | `53f076e` #17, `e7b3180` #22 | Parser scaffolded without a body, then with unused variables and no `return` | Three consecutive commits did not compile (33 minutes) |
| D-03 | `df6bbea` #18 | `a30975a` #19 (build), `e9dc9a3` #23 (output) | `read` body commented out; `fileContent` left unused | Commit did not compile; `read` then produced no output for about 19 minutes |
| D-04 | `7d7e0ca` #43 | `c2ab778` #47 | `data.json` committed without commas | JSON import failed until corrected |
| D-05 | `49588a8` #49 | `0e34ea9` #54 | `value.(string)` on arbitrary JSON values | Panic on numbers, maps, null |
| D-06 | `1c25f3e` #71 | `030a3e8` #73 | Per-rune `done` flag reset; no colon guard | Split logic did not do what it claimed; potential panic |
| D-07 | `5d225a6` #76 | `b6c6b6f` #81 | WAL lock released before `WriteString` | No mutual exclusion around the write |
| D-08 | `fd0de23` #80 | `b6c6b6f` #81 | `wal.mu.Unlock()` without `Lock` | Guaranteed panic on first write |
| D-09 | `d50a86d` #84 | `bfa974c` #85 | Read path locked and never unlocked | Deadlock if another locked operation followed |
| D-10 | `36fef1f` #94 | `fc6305b` #95 | Used `hash/fnv` and `io` without imports | Commit did not compile |
| D-11 | `36fef1f` #94 | `f2e1bfd` #101 | Checksum covered only the key | Value corruption undetected |
| D-12 | `c53b857`–`ad7b809` #105–#109 | `ea07671` #110 | Reader rewrite deleted the old path before the new one compiled; missing imports, calls to removed `extractRecords`/`readAllWAL`, stale `rec.DB`/`rec.Bucket` fields | Five consecutive commits did not compile (~14 minutes) |
| D-13 | `063ee82` #124 | `11db769` #125, `0aaeb6c` #126 | Checksum changed to include nanos but call sites and reader were not updated together | Two commits did not compile; new records failed verification for ~2 minutes; after #126 all earlier records also fail verification (C-06) |
| D-14 | `4770384` #128 | `6d9796b` #133 | `fmt.Errorf` without a format directive | Misleading error; `go vet` warning |
| D-15 | `b7e785c` #135 | `c7e7bc1` #139 | `scanWAL(..., nil)` then `db.updateDb` | `history` panicked on any non-empty WAL |
| D-16 | `b7e785c` #135 | `c7e7bc1` #139 | `wal.mu` held while taking `db.mu` | Reverse lock order; potential deadlock |
| D-17 | `4c16777` #131 / `cc9b516` #138 | `c7e7bc1` #139 | Snapshot written but never loaded; errors ignored; partial overwrite possible | Snapshot could lose state; feature inactive |
| D-18 | `303dd4e` #137 | `c7e7bc1` #139 | `load` command silently returned success | Misleading CLI behaviour |
| D-19 | `334dbf7`–`d8373d2` #57–#59 | `4701fe2` #60 | Rename to `WALname` left call sites referencing `DbName` | Three commits did not compile (~4 minutes) |
| D-20 | `063ee82`–`11db769` #124–#125 | `0aaeb6c` #126 | Checksum signature gained a parameter but call sites were updated one at a time | Two commits did not compile (~2 minutes) |
| D-21 | `4770384` #128 | `88c55f5` #129 | `CreateDB` referenced before it was defined | Commit did not compile (17 seconds) |

### 10.2 Compatibility breaks

These are deliberate schema changes, not accidental defects. They are recorded because they determine which historical files a given build can read.

| ID | Commit | Break | Effect |
|---|---|---|---|
| C-01 | `6f8b452`, `1aab938` (#61, #63) | Removed version from each record; moved it to the filename | alpha.0 and alpha.1 readers/writers are mutually incompatible |
| C-02 | `a02373f` #114 | Removed formatted-timestamp parsing | Early alpha.2 records are rejected as corrupt |
| C-03 | `590d634` #116 | Newline moved from suffix to prefix | Files no longer end with a newline; external line-oriented tools may care |
| C-04 | `c7e7bc1` #139 | Added sequence number to WAL and checksum | alpha.2 WAL records cannot be read by alpha.3; filenames isolate them |
| C-05 | `c7e7bc1` #139 | Snapshot became `{seq, layout}` plus checksum; old name `snapShot.aakku` abandoned | Plain-JSON snapshots from alpha.2 are not loaded by alpha.3 |
| C-06 | `063ee82`–`0aaeb6c` #124–#126 | Checksum coverage extended to include the timestamp | Every earlier alpha.2 record fails verification; the history refreshed in #127 drops `CatDb CatBucket`, `HumainDb hobby`, and `HumainDb aakkuTapari` |

### 10.3 Open risks at `HEAD` (`d434d69`)

| ID | Area | Risk | Why it matters |
|---|---|---|---|
| O-01 | Concurrency | No inter-process file lock | Two CLI invocations can assign duplicate sequence numbers, interleave writes, and race on the shared `.tmp` snapshot name. Correctness assumes one process. |
| O-02 | Growth | WAL is never compacted or truncated | Disk usage grows forever; `history` scans every record ever written. |
| O-03 | Derived data | History exists only in the WAL | Deleting or losing the WAL loses history even if a snapshot exists; the snapshot stores only latest values. |
| O-04 | Integrity | FNV-128a is non-cryptographic | Detects accidents, not tampering. Anyone with write access can forge a record and recompute the hash. |
| O-05 | Ordering | Sequence numbers are trusted, not validated | A duplicate, gap, or out-of-order record can cause later records to be skipped after a snapshot load. |
| O-06 | Recovery | Snapshot failure triggers full WAL replay | Safe only while the full WAL is present; future compaction must preserve everything needed by the snapshot fallback. |
| O-07 | Quality | No tests, CI, lint, or build gate | Sixteen commits (11%) did not compile in seven windows; several runtime regressions also survived until manual runs. Nothing prevents recurrence. |
| O-08 | Artifacts | `main` binary is stale (last built 2026-10-14) | Running it executes pre-alpha.3 code and may write incompatible files. Build from source. |
| O-09 | Data model | All values are strings; JSON import is lossy | Nested structures are flattened to Go formatting; types and structure cannot be recovered. |
| O-10 | Security | No encryption or access control; the WAL is created with mode 0644 | The database stores personal data (`AakkusPersonalDb`) and is readable by other local users. |
| O-11 | Durability | History output is not fsynced or atomic | A crash during `history` can leave a partial derived file; the next run regenerates it. |

### 10.4 Buildability gaps

Every commit's `main.go` was extracted and compiled in isolation with the current Go toolchain. Sixteen commits did not build. The failures are language-level (missing bodies, unused variables, undefined names), not toolchain drift, so they would have failed with any modern Go compiler.

| Window | Commits | Count | Cause | First buildable commit | Window length |
|---|---|---:|---|---|---|
| #14–#16 | `af291d4`, `0241167`, `0ca97d6` | 3 | Parser function declared without a body, then with unused variables and no return | `53f076e` #17 | 33m 00s |
| #18 | `df6bbea` | 1 | `read` body commented out; `fileContent` unused | `a30975a` #19 | 0m 28s |
| #57–#59 | `334dbf7`, `db16fe0`, `d8373d2` | 3 | Rename left call sites referencing `DbName` | `4701fe2` #60 | 3m 48s |
| #94 | `36fef1f` | 1 | `hash/fnv` and `io` used without imports | `fc6305b` #95 | 0m 19s |
| #105–#109 | `c53b857`, `201c806`, `1561c6a`, `5907b70`, `ad7b809` | 5 | Missing imports, calls to removed `extractRecords`/`readAllWAL`, stale `rec.DB`/`rec.Bucket` fields | `ea07671` #110 | 14m 07s |
| #124–#125 | `063ee82`, `11db769` | 2 | Checksum signature changed; call sites updated one at a time | `0aaeb6c` #126 | 1m 54s |
| #128 | `4770384` | 1 | `CreateDB` referenced before the next commit defined it | `88c55f5` #129 | 0m 17s |
| **Total** |  | **16** |  |  |  |

The two long windows are the instructive ones. The parser sequence (#14–#16) shows how a half-written function can persist across three commits when each one is committed before the previous state is runnable. The reader sequence (#105–#109) shows the cost of deleting a working path before its replacement compiles: the missing imports were easy to fix, but the calls to removed `extractRecords`/`readAllWAL` and the renamed `rec.DB`/`rec.Bucket` fields kept the tree broken for five commits.

---

## 11. Architecture at `HEAD`

### 11.1 Components and entry points

| Component | Location | Responsibility |
|---|---|---|
| CLI dispatch | `run` | Parses `write`/`read`/`history`, validates arguments, returns errors |
| Record construction | `newRecord`, `validateHeaderField`, `checksum` | Trims and validates fields, captures the timestamp, and computes FNV-128a; the sequence is supplied by `Set` |
| WAL | `HoneyberriWAL`, `OpenWAL`, `appendRecord`, `scanWAL`, `parseLine`, `encode` | Append+fsync, full scan, integrity checking, corrupt-line counting |
| In-memory DB | `HoneyberriDB`, `OpenDb`, `Set`, `Get`, `PrintAll`, `applyLocked` | Snapshot load, WAL replay, writes, memory reads |
| History | `ReadHistory`, `writeHistory` | Full WAL scan grouped by `(tapari, duna, key)`; stdout plus file |
| Snapshot | `DBsnapShot`, `loadSnapshotLocked`, `sumHex`, `syncDir` | Checksummed JSON, atomic replace, startup load |

### 11.2 Startup and shutdown

1. `OpenWAL` opens or creates `HoneyberriDB0.0.1-alpha.3.aakku` in the current working directory.
2. `OpenDb` acquires `db.mu`, loads `Snapshot.HoneyberriDB0.0.1-alpha.3.aakku` if present, verifies the checksum, and initialises `lastSeq`/`snapSeq` from the snapshot's `seq`.
3. It scans the WAL under `wal.mu` and applies only records with `seq > lastSeq`.
4. A corrupt snapshot is reported on stderr and ignored: memory resets and the full WAL is replayed.
5. After the command, `CloseDb` writes a snapshot if `lastSeq != snapSeq` (including the case where a WAL-only database has no snapshot yet).
6. The WAL is closed last.

### 11.3 Write path

`Set` holds `db.mu` for the entire operation:

1. Validate and normalise `tapari`, `duna`, `key`, `value`; assign `seq = lastSeq + 1`; capture `time.Now()` once.
2. Compute the checksum over `seq`, all four fields, and `UnixNano`.
3. Encode as `\n<seq> <tapari> <duna> <key> ;;<value>;; <unixnano> <hash>`.
4. Append to the WAL and `fsync`.
5. Apply to memory, set `lastSeq`, return the record.

Because the WAL append happens before the memory update and both are under `db.mu`, memory order matches WAL order for a single process.

### 11.4 Read and history paths

- `read` calls `PrintAll`, which copies the layout into a buffer under `db.mu.RLock()` and writes outside the lock. No WAL scan occurs.
- `history` calls `db.wal.ReadHistory`, which scans the entire WAL under `wal.mu`, groups values oldest-first, prints to stdout and writes `History.HoneyberriDB0.0.1-alpha.3.aakku`. Corrupt lines are counted and reported on stderr.
- `Get` exists but is not exposed by the CLI.

### 11.5 On-disk formats

**WAL** (one record per line, each record prefixed by `\n`):

```text
<seq> <tapari> <duna> <key> ;;<value>;; <unixnano> <fnv128a-hex>
```

Example:

```text
7 NepalTapari ButwalDuna cat ;;cat;; 1792404720970370000 80dbc84a32370dfafe8128980bf2c1f1
```

**Snapshot** (`<json>` then a newline, then the FNV-128a hex of the JSON):

```json
{"seq":7,"layout":{"NepalTapari":{"ButwalDuna":{"cat":{"v":"cat","t":1792404720970370000}}}}}
<fnv128a-hex-of-the-json>
```

**History** (derived; sorted by tapari, duna, key):

```text
NepalTapari ButwalDuna
cat:dog,cat,cat
```

**Current filenames** (all in the working directory):

- `HoneyberriDB0.0.1-alpha.3.aakku` — WAL
- `Snapshot.HoneyberriDB0.0.1-alpha.3.aakku` — snapshot
- `History.HoneyberriDB0.0.1-alpha.3.aakku` — history export

### 11.6 Concurrency and durability

- **In-process:** `db.mu` protects layout and sequence; `wal.mu` protects the shared file handle and offset. Lock order is always `db.mu → wal.mu`; `ReadHistory` takes only `wal.mu`; `DBsnapShot` takes `snapMu` then `db.mu`; no path takes them in reverse.
- **Cross-process:** None. There is no `flock` or lock file.
- **WAL durability:** One `WriteString` plus `File.Sync()` per record.
- **Snapshot durability:** Write temp file, `Sync`, close, `Rename`, best-effort directory `Sync`.
- **History durability:** Best effort; the file is a derived cache and is recreated on the next run.

### 11.7 Versioning and compatibility

Filenames are the compatibility boundary. `WALversion` selects the WAL and history files; `SoftwareVersion` selects the snapshot. Bumping a version starts a new dataset rather than migrating the old one. At `HEAD`, no alpha.3 data files are committed, so a first run in the repository starts empty; all committed `.aakku` files are historical artifacts from earlier generations.

### 11.8 Operational notes

- All data paths are relative to the current working directory. Run experiments in a temporary directory to avoid creating untracked `.aakku` files in the repository.
- Even `read` and `history` create the WAL file if it is missing, because `OpenWAL` runs before dispatch.
- The committed `main` binary is stale; build with `go build -o /tmp/honeyberri main.go` or use `go run main.go`.
- There is no `go.mod`; `go build .` fails, while `go build main.go` works.

---

## 12. Engineering Lessons

1. **Keep every commit buildable.** Sixteen commits across seven windows failed to compile. The causes are mundane — missing imports, unused variables, stale field names, a changed function signature, a function declared without a body. The longest window lasted 33 minutes; the longest sequence covered five commits. Every one would have been caught by `go build main.go` before committing. A one-command pre-commit gate is the highest-value change this repository could make.
2. **Separate mechanical refactors from behaviour changes.** `9fc0e80` moved a function and silently moved the call site with it. A pure move plus a separate behavioural commit would have made the mistake obvious.
3. **Use one implementation for writer and reader integrity logic.** The checksum drifted because the writer and reader computed it in different places (`36fef1f`→`f2e1bfd`, `063ee82`→`0aaeb6c`). The shared `checksum` function introduced in `6b21c1b` is the right pattern.
4. **Persist only state that can be fully reconstructed.** The snapshot experiments wrote a partial in-memory DB before loading the WAL into it. `c7e7bc1` fixed this by loading the snapshot first and replaying the remainder.
5. **Define on-disk formats before committing fixtures.** History changed format three times in 26 minutes, leaving inconsistent committed examples. Generated files should include regeneration instructions or be excluded from version control.
6. **Versioned filenames are a migration strategy, but only if isolation is acceptable.** The project repeatedly avoided migrations by changing the filename. That keeps old data safe but makes it invisible to the new build.
7. **Document and enforce lock order structurally.** The `wal.mu → db.mu` inversion existed only because a scan callback had a side effect. Keeping scanning free of DB mutation (as `c7e7bc1` does) is safer than relying on comments.
8. **Test unhappy paths.** Snapshot-not-found, corrupt checksum, nil DB, duplicate sequence, and concurrent processes are exactly the cases the project did not exercise. The happy path was manually verified; the failure paths were discovered by reading the code.
9. **Do not commit build artifacts.** The `main` binary was useful once and has been misleading ever since.
10. **Return errors; do not hide them in deferred calls.** The deferred snapshot ignored its error, and the `load` command returned success without doing anything. Both were corrected only when the author redesigned the lifecycle.

---

## Appendix A — Complete Commit Index

All 141 commits reachable from `main` at `d434d69`, in chronological order. Times are author dates in NPT. Messages are verbatim, including original spelling. Phase keys: **P0** flat-file prototype, **P1** WAL identity/locking, **P2** WAL v2, **P3** alpha.2 hardening, **P4** memory/snapshots, **P5** alpha.3/docs.

| # | Date and time (NPT) | Commit | Original message | Phase |
|---:|---|---|---|---|
| 1 | 2026-10-12 00:46:49 | `3c18809` | initial code to write/append in to file adn read all file | P0 |
| 2 | 2026-10-12 00:58:47 | `fcbdf32` | bucked to extract final data | P0 |
| 3 | 2026-10-12 00:59:15 | `2db767b` | go style packing | P0 |
| 4 | 2026-10-12 01:03:22 | `d99dd22` | this make code just more redable | P0 |
| 5 | 2026-10-12 01:04:31 | `5a008b7` | just formatign output | P0 |
| 6 | 2026-10-12 01:07:26 | `70e5f82` | soem stupid formating | P0 |
| 7 | 2026-10-12 01:09:25 | `2b2480a` | extractRecords loops through the raw file data and pulls out each block wrapped in {} | P0 |
| 8 | 2026-10-12 01:12:48 | `8ce03c2` | simple nam change + feeding extractRecords the daata from file ie filecontent | P0 |
| 9 | 2026-10-12 01:13:28 | `0c0c23d` | Justo come comments | P0 |
| 10 | 2026-10-12 01:15:38 | `5acef8f` | formating in reading data | P0 |
| 11 | 2026-10-12 01:17:35 | `8320e73` | eg data we get form this func | P0 |
| 12 | 2026-10-12 01:21:00 | `9fc0e80` | seperatign write to its function to de-claugher main func | P0 |
| 13 | 2026-10-12 01:21:32 | `00a6072` | xd moved write to its proper plavce | P0 |
| 14 | 2026-10-12 01:26:08 | `af291d4` | well lest try making this a type so that we cna return it more fluently | P0 |
| 15 | 2026-10-12 01:43:31 | `0241167` | cause it was creatign confusion with variable naming below in code | P0 |
| 16 | 2026-10-12 01:44:00 | `0ca97d6` | trying ti rxtreact version,key and value from records | P0 |
| 17 | 2026-10-12 01:59:08 | `53f076e` | seperating by : and , | P0 |
| 18 | 2026-10-12 02:04:44 | `df6bbea` | clearing debugging | P0 |
| 19 | 2026-10-12 02:05:12 | `a30975a` | name change | P0 |
| 20 | 2026-10-12 02:05:30 | `f1e0305` | proper implementation more work needed here | P0 |
| 21 | 2026-10-12 02:20:21 | `b0746b2` | actually map already works no need for slice of map | P0 |
| 22 | 2026-10-12 02:20:43 | `e7b3180` | proper implementation | P0 |
| 23 | 2026-10-12 02:23:31 | `e9dc9a3` | var name change + clean code | P0 |
| 24 | 2026-10-12 02:24:10 | `2195a90` | formating | P0 |
| 25 | 2026-10-12 02:24:31 | `a520e84` | same | P0 |
| 26 | 2026-10-12 02:28:19 | `b016916` | not readig of version mish match | P0 |
| 27 | 2026-10-12 02:30:11 | `9368c82` | seperating read loigc from main func to declaughter | P0 |
| 28 | 2026-10-12 02:37:16 | `7af73d6` | why to keep { and } then we have to eventiually remove it | P0 |
| 29 | 2026-10-12 02:40:29 | `2313eed` | testig with version vhange | P0 |
| 30 | 2026-10-12 02:55:26 | `f49e8e7` | read history | P0 |
| 31 | 2026-10-12 03:05:52 | `195a49d` | proper jistory display function | P0 |
| 32 | 2026-10-12 03:06:24 | `98ccce7` | cleanup | P0 |
| 33 | 2026-10-12 03:11:39 | `51456bb` | cleanup | P0 |
| 34 | 2026-10-12 03:11:56 | `1658ae9` | peoper error handeling while writig in console | P0 |
| 35 | 2026-10-12 03:31:40 | `fae00e1` | proper extract historytfunctipon | P0 |
| 36 | 2026-10-12 03:33:51 | `38f44ab` | removed useless else block | P0 |
| 37 | 2026-10-12 03:53:12 | `4069d7c` | beautifing terminal output | P0 |
| 38 | 2026-10-12 03:54:13 | `210e122` | compressing extracted history to save disk space | P0 |
| 39 | 2026-10-12 03:54:30 | `eeca524` | compressed | P0 |
| 40 | 2026-10-12 03:56:14 | `108e099` | cleanup | P0 |
| 41 | 2026-10-12 03:57:23 | `2f46d19` | thers no need of } at last so replaced { with : whats saves closign bracket } | P0 |
| 42 | 2026-10-12 03:58:10 | `bc841d7` | formaitng | P0 |
| 43 | 2026-10-12 04:01:52 | `7d7e0ca` | will write parse JSON fucn to parse it | P0 |
| 44 | 2026-10-12 04:01:59 | `386e887` | hsirrfj chnae | P0 |
| 45 | 2026-10-12 04:27:47 | `bfcc211` | some json sanitization | P0 |
| 46 | 2026-10-12 04:28:37 | `d016853` | cleanups | P0 |
| 47 | 2026-10-12 04:29:55 | `c2ab778` | valid json | P0 |
| 48 | 2026-10-12 04:47:06 | `76a1b34` | proper display func | P0 |
| 49 | 2026-10-12 04:47:23 | `49588a8` | properly passign args to write db func | P0 |
| 50 | 2026-10-12 04:47:48 | `208d3a1` | read from json | P0 |
| 51 | 2026-10-12 04:48:00 | `18e0ff1` | rad from json | P0 |
| 52 | 2026-10-12 04:51:37 | `b86268a` | efficient code | P0 |
| 53 | 2026-10-12 04:57:16 | `533bc23` | giving other value type | P0 |
| 54 | 2026-10-12 04:57:29 | `0e34ea9` | this converts everythign into string | P0 |
| 55 | 2026-10-12 04:57:34 | `8a64760` | new db | P0 |
| 56 | 2026-10-12 05:00:57 | `351ca7e` | giging differen ttypes of input | P0 |
| 57 | 2026-10-13 02:39:00 | `334dbf7` | thats not DB thats wal | P1 |
| 58 | 2026-10-13 02:40:49 | `db16fe0` | official DB name is honey Berry DB | P1 |
| 59 | 2026-10-13 02:41:53 | `d8373d2` | fashion my hehe | P1 |
| 60 | 2026-10-13 02:42:48 | `4701fe2` | name update on overall codeBase | P1 |
| 61 | 2026-10-13 02:47:30 | `6f8b452` | Since WAL will have version on it file name so thers no point on printing version on each entries | P1 |
| 62 | 2026-10-13 02:49:53 | `f6f64d8` | the history extraction will also follow sam epattern | P1 |
| 63 | 2026-10-13 02:51:15 | `1aab938` | thers no poin tof placing version num in history extraction | P1 |
| 64 | 2026-10-13 03:00:00 | `8bc6503` | now we dont need this | P1 |
| 65 | 2026-10-13 03:00:24 | `1e69acd` | formating | P1 |
| 66 | 2026-10-13 03:02:45 | `4d3deba` | same as sanitizing recorda follow 8bc65036 | P1 |
| 67 | 2026-10-13 03:03:29 | `bcfaf47` | we dont need that anymore | P1 |
| 68 | 2026-10-13 03:05:20 | `b1c7ef2` | atleast for now | P1 |
| 69 | 2026-10-13 03:08:59 | `a2193da` | new WAL file styles weeeeeeeeeeeeeeee | P1 |
| 70 | 2026-10-13 03:46:04 | `86ca102` | reading json | P1 |
| 71 | 2026-10-13 03:46:10 | `3ef0fac` | new history | P1 |
| 72 | 2026-10-13 03:46:22 | `1c25f3e` | TODO | P1 |
| 73 | 2026-10-13 22:46:21 | `030a3e8` | this cuts string in exactly 2 halfs then ":" 1st comes | P1 |
| 74 | 2026-10-13 22:52:46 | `bbe7b4b` | graceful cut for building history | P1 |
| 75 | 2026-10-13 22:53:57 | `b53008f` | new history fun works | P1 |
| 76 | 2026-10-13 23:12:06 | `220e94e` | properly rename | P1 |
| 77 | 2026-10-13 23:12:39 | `5d225a6` | locking WAL | P1 |
| 78 | 2026-10-14 00:32:35 | `fd350a5` | WAL struct + Constructor to opeanWAl | P1 |
| 79 | 2026-10-14 00:40:02 | `33e7436` | instance of WAL | P1 |
| 80 | 2026-10-14 00:41:12 | `fd0de23` | function --> method all binted with WAL | P1 |
| 81 | 2026-10-14 00:41:56 | `b6c6b6f` | proper wal lock accurance | P1 |
| 82 | 2026-10-14 00:42:06 | `daa3bc3` | formating | P1 |
| 83 | 2026-10-14 01:17:09 | `3579fc6` | f | P1 |
| 84 | 2026-10-14 01:17:54 | `d50a86d` | proper method tied up to WAL | P1 |
| 85 | 2026-10-14 01:19:39 | `bfa974c` | Proper ReadAllWAL method | P1 |
| 86 | 2026-10-14 01:23:46 | `1470c69` | same to read history | P1 |
| 87 | 2026-10-14 02:04:51 | `df95a4e` | seperated read part | P1 |
| 88 | 2026-10-14 02:05:17 | `98bf4f6` | new data+historyOnThatData | P1 |
| 89 | 2026-10-14 02:20:03 | `a535928` | called existing readAll func | P1 |
| 90 | 2026-10-14 02:20:17 | `df1b466` | newhistoryplusbuild | P1 |
| 91 | 2026-10-14 02:35:24 | `fe3a9de` | that was un-used | P1 |
| 92 | 2026-10-14 04:26:27 | `a593a41` | yup we makign new version | P2 |
| 93 | 2026-10-14 04:26:39 | `7efee1e` | for loggin langauges hehe | P2 |
| 94 | 2026-10-14 04:32:30 | `36fef1f` | thats new logging pattern | P2 |
| 95 | 2026-10-14 04:32:49 | `fc6305b` | new packages, new load , bigger inary huhuhu | P2 |
| 96 | 2026-10-14 05:05:53 | `a3921e6` | time+trinspace | P2 |
| 97 | 2026-10-14 05:22:58 | `cfa888a` | encodinf bytes into redable hex | P2 |
| 98 | 2026-10-14 05:23:12 | `7d73c13` | new WAL loggin g system | P2 |
| 99 | 2026-10-14 05:31:21 | `b0aca5a` | in case error handeler panics ti leaves wal reding locked forever in process | P2 |
| 100 | 2026-10-14 05:31:45 | `a264724` | name change hehe | P2 |
| 101 | 2026-10-14 05:36:25 | `f2e1bfd` | proper implementation of checksum | P2 |
| 102 | 2026-10-14 05:48:11 | `bd7c981` | after check sum | P2 |
| 103 | 2026-10-14 05:48:35 | `7e5325f` | using unix nano time format | P2 |
| 104 | 2026-10-14 05:49:32 | `6d95775` | checksum remains same, just logging format changes | P2 |
| 105 | 2026-10-14 18:13:53 | `c53b857` | new way to read new WAL style | P2 |
| 106 | 2026-10-14 18:21:24 | `201c806` | no more needed | P2 |
| 107 | 2026-10-14 18:21:58 | `1561c6a` | depricated, no more needed | P2 |
| 108 | 2026-10-14 18:26:49 | `5907b70` | new imports | P2 |
| 109 | 2026-10-14 18:27:09 | `ad7b809` | proper name in context of HoneyBerriDB | P2 |
| 110 | 2026-10-14 18:28:00 | `ea07671` | Re-impleementation of ReadWal and making History through wal | P2 |
| 111 | 2026-10-16 23:15:18 | `e9f9674` | nothing | P3 |
| 112 | 2026-10-16 23:18:02 | `14248d6` | Auto commit + push script ran from terminal | P3 |
| 113 | 2026-10-16 23:47:47 | `6b21c1b` | FIx/Changes: | P3 |
| 114 | 2026-10-17 00:04:32 | `a02373f` | Removed backward compataability to read old time formant, insted now we only read unixnano fornat only and it s fixed | P3 |
| 115 | 2026-10-17 00:04:43 | `dab5f96` | new entry nothign | P3 |
| 116 | 2026-10-17 00:42:36 | `590d634` | I have a BIG BRAIN | P3 |
| 117 | 2026-10-17 00:46:32 | `1a3cbf7` | moved software version below | P3 |
| 118 | 2026-10-17 00:47:16 | `5c37017` | Added new WAL versoin cause it cant be same as DB version/software Version | P3 |
| 119 | 2026-10-17 00:47:40 | `204fd99` | Proper wall name | P3 |
| 120 | 2026-10-17 00:47:51 | `933bba9` | proper history name | P3 |
| 121 | 2026-10-17 00:53:56 | `8ad29a0` | checking if it formats corectly | P3 |
| 122 | 2026-10-17 00:54:59 | `1f5b149` | new file name hehehe | P3 |
| 123 | 2026-10-17 01:08:46 | `a4b2805` | Merge pull request #1 from aakku106/0.0.1-alpha.2 | P3 |
| 124 | 2026-10-18 00:25:16 | `063ee82` | // checksum covers every field of the record except the hash itself. | P4 |
| 125 | 2026-10-18 00:26:17 | `11db769` | take the time once and use it for both fields | P4 |
| 126 | 2026-10-18 00:27:10 | `0aaeb6c` | verify with the parsed nano | P4 |
| 127 | 2026-10-18 00:31:55 | `814c401` | new history+WAL log data with complete checkSum | P4 |
| 128 | 2026-10-19 17:47:51 | `4770384` | creatign ans sendign db refrence | P4 |
| 129 | 2026-10-19 17:48:08 | `88c55f5` | implementation of in memory DB | P4 |
| 130 | 2026-10-19 17:48:13 | `d1095ed` | new inputs | P4 |
| 131 | 2026-10-19 18:14:42 | `4c16777` | snapshottingdb | P4 |
| 132 | 2026-10-19 18:15:40 | `44ac017` | used method | P4 |
| 133 | 2026-10-19 18:20:09 | `6d9796b` | just ovinf thingup/down hehe | P4 |
| 134 | 2026-10-19 18:20:31 | `211172e` | newSnapshotfeature | P4 |
| 135 | 2026-10-19 18:56:41 | `b7e785c` | snapshottig while reading all wal | P4 |
| 136 | 2026-10-19 18:56:53 | `aff3238` | new snapshot+data | P4 |
| 137 | 2026-10-19 22:59:08 | `303dd4e` | load case | P4 |
| 138 | 2026-10-19 23:23:20 | `cc9b516` | loadign snapshort | P4 |
| 139 | 2026-10-19 23:54:58 | `c7e7bc1` | new wal+software version with sequence number and proper in memory db load,read,write and snapshot | P5 |
| 140 | 2026-10-19 23:55:53 | `fa6e13a` | found out opencode alows some free tear aimodels just testing them to write some doc aand init file | P5 |
| 141 | 2026-10-19 23:59:51 | `d434d69` | soem wordings hehe | P5 |

---

## Appendix B — Tags, Branches, and Artifacts

### B.1 Tags

| Tag | Type | Target commit | Timestamp | Meaning in this history |
|---|---|---|---|---|
| `v0.0.1-alpha.0` | Annotated (tag object `49a1ae5`) | `351ca7e` | Tag created 2026-10-13 01:11:58; target dated 2026-10-12 05:00:57 | Prototype baseline before the WAL/format rework. Tag message: *"This is its 1st release, i am goign to make some bracking changes so thought its good to tag."* |
| `v0.0.1-alpha.1` | Lightweight | `fe3a9de` | 2026-10-14 02:35:24 | End of the first WAL object/locking phase. |
| `v0.0.1-alpha.2` | Lightweight | `cc9b516` | 2026-10-19 23:23:20 | Snapshot-loading commit; placed during the transition to alpha.3, not at the PR #1 merge. |

Lightweight tags have no tagger metadata; the timestamp shown for them is the target commit's author date. Only `v0.0.1-alpha.0` is an annotated tag object.

### B.2 Branches and refs

| Ref | Tip | Relationship |
|---|---|---|
| `refs/heads/main` | `d434d69` | Documented line; 141 commits |
| `HoneyberriDB/main` | `d434d69` | Remote tracking branch |
| `HoneyberriDB/0.0.1-alpha.2` | `1f5b149` | Merged into `main` by `a4b2805` (PR #1); no unique commits remain |
| `refs/stash` | `a9bf0dd` | Local WIP snapshot (*"On main: ome changes"*); excluded from project history |

The Git remote is named `HoneyberriDB`, not `origin`.

### B.3 Repository artifacts at `HEAD`

| Path | Status | Notes |
|---|---|---|
| `main.go` | Current | 587 lines; the entire implementation |
| `main` | Stale | macOS/arm64 binary; last rebuilt in `1561c6a` on 2026-10-14, before snapshots and alpha.3 |
| `AakkusPersonalDb.aakku` | Historical | alpha.0 flat-file prototype; not read by current code |
| `HoneyberriDB0.0.1-alpha.1.aakku` | Historical | alpha.1 brace-format WAL |
| `HoneyberriDB0.0.1-alpha.2.aakku` | Historical | alpha.2 WAL with mixed formatted/numeric timestamps |
| `History0.0.1-alpha.1.aakku`, `History0.0.1-alpha.2.aakku`, `History.HoneyberriDB0.0.1-alpha.2.aakku` | Historical | Generated history exports from earlier formats |
| `history.aakku` | Historical | Early generic history export from the alpha.0/alpha.1 era |
| `snapShot.aakku` | Historical | Plain-JSON snapshot from the alpha.2 experiments; not the current snapshot format |
| `data.json` | Fixture | JSON seed used by the removed `parse` command |
| `AGENTS.md` | Current | Agent-facing build/run/data-file instructions |
| `HoneyberriDB0.0.1-alpha.3.aakku` | Not committed | Created by the current build in the working directory |
| `Snapshot.HoneyberriDB0.0.1-alpha.3.aakku` | Not committed | Created by `CloseDb` when the sequence advances |
| `History.HoneyberriDB0.0.1-alpha.3.aakku` | Not committed | Created by the `history` command |

No alpha.3 data files are committed. A first run of the current code in a clean checkout therefore starts with an empty database; all committed `.aakku` files belong to earlier generations and are ignored.

---

## Appendix C — Method, Classification, and Caveats

### C.1 Sources

- Full patch history: `git log --reverse -p` over all 141 commits reachable from `main` at `d434d69`.
- Commit metadata: hashes, author dates, subjects, and bodies.
- Annotated tag object for `v0.0.1-alpha.0`; peeled targets for all tags.
- Merge commit metadata and parents for `a4b2805`.
- Tree contents at selected commits (`cc9b516`, `b7e785c`, `c53b857`, `36fef1f`, `0ca97d6`) to verify historical claims.
- Buildability: every commit's `main.go` was extracted to a temporary directory and compiled with the current Go toolchain using `go build main.go`. The sixteen failures are listed in Section 10.4.
- Final source and committed fixtures at `HEAD`.

### C.2 Classification

- **Confirmed defect:** a later commit explicitly changes the code in a way that corrects the behaviour. The fixing commit is cited.
- **Potential/latent defect:** a code-level flaw that follows from the implementation but may not have been triggered before the code changed. It is labelled as such.
- **Compatibility break:** a deliberate format or filename change that makes older data unreadable by the newer build. These are not counted as defects.
- **Open risk:** a property of the current design that remains true at `HEAD`.

### C.3 Caveats

- Intent statements are inferred from commit messages and diffs. The author was not interviewed; where a message is terse or playful, the analysis relies on the code change and labels the inference.
- The repository has no tests, CI, issue tracker entries, or runtime telemetry, so no runtime defect in this document is backed by a production incident. The "Consequence" column describes the logical effect of the code. Build failures, by contrast, were reproduced directly.
- Historical bug windows are measured between the introducing and fixing commits. Whether a binary was built and run during a window is unknown. Build windows are measured from the first failing commit to the first following commit that builds; because a commit can be broken by earlier work, attribution is to the introducing cause.
- Buildability was checked with the Go toolchain available on 2026-10-20. The failures are language-level (missing bodies, unused variables, undefined names), not version-specific.
- Dates are author dates in NPT. The `v0.0.1-alpha.0` tag date differs from its target commit date and is reported separately.
- Commit messages in Appendix A are reproduced verbatim to preserve traceability; the prose paraphrases them in standard English.
- The stash entry is a local WIP snapshot, not part of the deliverable history, and is excluded from the 141-commit count.
- This document describes `d434d69`. Later commits, uncommitted work, and untracked runtime files are out of scope.

---

*End of document.*
