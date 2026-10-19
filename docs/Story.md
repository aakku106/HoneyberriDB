# HoneyberriDB — A Story in Six Chapters

*The short history of a personal database, told from its commits.*

> *"This is its 1st release, i am goign to make some bracking changes so thought its good to tag."*
> — the annotated tag message for `v0.0.1-alpha.0`

Between 12 and 19 October 2026, Adarasha Gaihre — who commits as `aakku106` — wrote 141 commits on a single Go file and called the result HoneyberriDB. No server, no dependencies, no build files: one `main.go`, the standard library, and a growing text file that remembers everything ever written to it.

This is the short version of that week and a half: not what every commit changed, but what the author was trying to do, and what the attempts cost. The full forensic account is in [`History.md`](History.md).

## 1. A database of one's own

The first commit says the goal plainly: *"initial code to write/append in to file and read all file."* That is all it says. The database is named for its first user — `AakkusPersonalDb.aakku`, after the author's nickname — and its first record is his own name. It was never meant to compete with Postgres. It was meant to be understood, owned, and kept.

The project name came later, and the author left a note explaining it: *"Checkout in internet about honeyberry its awsome and you might also get why i choosed name."* The honeyberry is a small, cold-hardy fruit. It is not a common name for software, which may be exactly why it fits.

The ambitions are visible in what the code refused to do. It refused a database driver, a server, a configuration file, and a package manager. Everything lives in one file in the working directory, and the file extension — `.aakku` — carries the author's name into every copy of the data.

## 2. The first night

The history begins at 00:46 on 12 October and does not pause until after 05:00. Fifty-six commits in four hours and fourteen minutes. Most are small: a rename, a format tweak, a fixture, a "hehe." But the shape of the whole project appears in that first night.

The storage format was a brace-wrapped record, `{version:key,value}`, appended to one file. `read` printed it. Then, at 02:55, the commit *"read history"* added the feature that defines HoneyberriDB: for every key, every value it has ever held, not just the latest one.

That is the author's central intention, and it never changed. A normal key-value store keeps the present. HoneyberriDB keeps the past, because the log was append-only from the very first commit anyway. History was not bolted on; it was already lying there.

The night also introduced the project's signature flaw. There was no build gate. A refactor sent `write` into the `read` branch and the program cheerfully reported success without saving anything. A parser was committed as a function declaration with no body. Three more commits later it still would not compile. The author fixed each one by hand, at speed, at two in the morning, and kept going.

## 3. Learning to say WAL

The next day has a commit title that sounds like a realization: *"thats not DB thats wal."* The file was never the database; it was a write-ahead log. Naming it correctly reorganized the code. The version moved out of every record and into the filename (`0.0.1-alpha.1`), which made the file self-describing without repetition. And the functions moved onto a `HoneyberriWAL` object with one open handle and one mutex.

Getting that mutex right took three wrong versions in one evening. The first lock was released before the write it was supposed to protect. The second was paired with an unlock that had no lock, which would have panicked on the first write. The third was right, and that design survives today. It is the most human part of the history: a concurrency lesson learned by walking into every wall.

The night ends with a `v0.0.1-alpha.1` tag and the first committed binary. The tag is lightweight, the binary is a Mac-only artifact that would soon be stale, but the gesture matters. The author was treating this as a real project.

## 4. A grammar for the log

Early on 14 October, the record format was replaced. The brace syntax could not hold spaces, delimiters, timestamps, or integrity data, so the log acquired a grammar:

```
<tapari> <duna> <key> ;;<value>;; <unixnano> <fnv128a>
```

The names are the most personal thing in the repository. A *tapari* is a plate woven from leaves; a *duna* is a small leaf bowl. In the database, a duna is a table of key/value pairs and a tapari is a collection of dunas — a namespace. The sample data says `NepalTapari ButwalDuna`, marrying the data model to a place and a household tradition. The author was not just building storage; he was furnishing it.

The checksum arrived in the same wave, and then had to grow: first it hashed only the key, then all fields. The first version proved that a value could be corrupted without the key changing, which is exactly the kind of thing a database author should learn from his own log.

Then came the reckoning. The reader was rewritten one afternoon, and for five consecutive commits the project did not compile. The old parser was deleted before its replacement existed; the reader called functions that were already gone and referenced fields that had been renamed. The fix, at 18:28, introduced `scanWAL`: one parser, one traversal, shared by `read` and `history`. The commit message is almost a thesis statement — *"Re-impleementation of ReadWal and making History through wal."* History does not need its own store. It is a reading of the log.

## 5. The hard pause

On 16 and 17 October, the author stopped adding and started hardening. One large commit, modestly titled *"FIx/Changes:"*, added argument validation, error returns, a buffered scanner, deterministic output, and `fsync` — with a comment that reads like a promise: *"don't return until it's on disk."*

It also removed things. The `parse` command went away, stricter validation went in, and old habits were cut loose. On 17 October the work was merged through GitHub as PR #1, and the `0.0.1-alpha.2` line was consolidated.

That is when the project paid its first real price. Supporting the old formatted timestamp was declared not worth the complexity, and records written in that format stopped being readable. The history file committed alongside the change shows the wound: it contains the values from numeric records but silently omits `Adarasha Gaihre`, a name that existed only in the old format. The author's own name was the first casualty of backward compatibility.

The same week produced the project's best commit title and its most serious reasoning:

> *"I have a BIG BRAIN"*
> *"Even if the app crasses in middle of writing, two records wont be corrupted, oly one will be corrupted, the next comming wouldn't be affected"*

The change was to move the newline from the end of a record to the beginning. If the process dies mid-write, the next record still starts on its own line, so one damaged record cannot poison the one after it. The joke and the engineering are the same thing: the log protects itself from its writer.

## 6. Memory and snapshots

The last chapter is about remembering where the database already is. Until 18 October, every run replayed the whole log. The author began building an in-memory copy of the current state and a JSON snapshot of it.

The first attempts are a small catalogue of how hard state is. The snapshot was written on every run — including reads, which could write an empty snapshot over a fuller one — and it was never loaded. The `history` command scanned the log with a nil database and crashed. A `load` command existed and did nothing. The snapshot was a photograph the program refused to look at.

On 18 October, the checksum was extended to cover the timestamp. This closed one hole and opened another: every record written before the change now failed verification. The fixture refresh that followed quietly dropped `CatDb CatBucket`, `hobby`, and `HumainDb aakkuTapari` from the history. The author did not roll back. He re-appended some of the values and kept going, which is perhaps the most honest database migration in the whole repository.

Then, at 23:54 on 19 October, the last commit arrived: *"new wal+software version with sequence number and proper in memory db load,read,write and snapshot."* Version `0.0.1-alpha.3` gave every record a sequence number, gave the snapshot a checksum and an atomic rename, loaded it at startup, and replayed only the records newer than it. The database finally knew where it had left off.

The final commit of the project is titled *"soem wordings hehe."* It fixes a sentence in the project's own instructions. The author's last act was to correct his writing about the code — a fitting end for someone whose database is, at heart, a device for keeping a faithful record.

## 7. What the story is about

The numbers are not flattering. Sixteen of the 141 commits did not compile. There are no tests, no CI, no lint, and no README. The committed binary is stale. The log grows forever; the history scan is linear; two processes can still collide. The full account, defect by defect, is in [`History.md`](History.md).

And yet the thing works. It writes durably, verifies what it reads, reconstructs its state from a checksummed snapshot and the records that followed, and can tell you every value a key has ever held. Built in eight days, in one file, with nothing but Go's standard library, it is a real — small, flawed, complete — database.

The intentions behind it seem to have been these. Learn how a database works by building the smallest one that could be true. Keep the history, because a record of change is more interesting than a snapshot of the present. Own the whole thing, down to the file extension, the lock order, and the names of the tables. And have some fun while doing it — enough fun to title a durability fix *"I have a BIG BRAIN"* and to name the namespace after a leaf plate.

HoneyberriDB is not the story of an expert building a product. It is the story of a student, far from the big database companies, deciding that the only way to understand a system was to write one down. The log only ever grows, and so, in the end, did he.

---

## The names, briefly

| Name | Meaning |
|---|---|
| **HoneyberriDB** | Named after the honeyberry; the author invites you to look it up |
| **Aakku / aakku106** | The author's nickname and handle |
| **`AakkusPersonalDb.aakku`** | The first database file; the author's own name and records |
| **tapari** | A plate woven from leaves; in the database, a namespace of dunas |
| **duna** | A leaf bowl; in the database, a table of key/value pairs |
| **WAL** | The write-ahead log; the append-only file that makes history possible |
| **`.aakku`** | The file extension that carries the author's name in every copy |

*This is the short telling. [`History.md`](History.md) is the long one: all 141 commits, the defect register, the compatibility breaks, and the architecture as it stands.*
