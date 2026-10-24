package honeyberridb

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	// "log"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	DBname          = "HoneyberriDB"
	SoftwareVersion = "0.0.1-alpha.3"
	WALversion      = "0.0.1-alpha.3"
	WALname         = DBname + WALversion + ".aakku" // We will add WAL. in front of DBname in next version
	SnapshotName    = "Snapshot." + DBname + WALversion + ".aakku"
	HistoryName     = "History." + DBname + WALversion + ".aakku"

	valueSep = ";;"
)

// ---------- WAL ----------

type HoneyberriWAL struct {
	mu   sync.Mutex // Mutex, not RWMutex: scanning Seeks the shared file offset
	file *os.File
}

func OpenWAL() (*HoneyberriWAL, error) {
	file, err := os.OpenFile(WALname, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}
	return &HoneyberriWAL{file: file}, nil
}

func (wal *HoneyberriWAL) Close() error { return wal.file.Close() }

// ---------- main ----------

// func main() {
// 	if err := realMain(); err != nil {
// 		log.Fatalln(err)
// 	}
// }

//	func realMain() error {
//		wal, err := OpenWAL()
//		if err != nil {
//			return err
//		}
//		defer wal.Close()
//
//		// OpenDb = load snapshot (if any) + replay WAL records newer than it.
//		db, skipped, err := OpenDb(wal)
//		if err != nil {
//			return err
//		}
//		WarnSkipped(skipped)
//
//		err = run(db, os.Args)
//		if cerr := db.CloseDb(); err == nil { // CloseDb writes a snapshot if anything changed
//			err = cerr
//		}
//		return err
//	}

func Open() (*HoneyberriDB, error) {
	wal, err := OpenWAL()
	if err != nil {
		return nil, err
	}
	// OpenDb = load snapshot (if any) + replay WAL records newer than it.
	db, skipped, err := OpenDb(wal)
	if err != nil {
		return nil, err
	}
	WarnSkipped(skipped)
	return db, nil
}
func (db *HoneyberriDB) Close() {
	db.wal.Close()
	db.CloseDb()
}

//
// func run(db *HoneyberriDB, args []string) error {
// 	if len(args) < 2 {
// 		return fmt.Errorf("usage: write|read|history")
// 	}
//
// 	switch args[1] {
// 	case "write", "w":
// 		if len(args) < 6 {
// 			return fmt.Errorf("usage: write <tapari> <duna> <key> <value...>")
// 		}
// 		rec, err := db.Set(args[2], args[3], args[4], strings.Join(args[5:], " "))
// 		if err != nil {
// 			return err
// 		}
// 		fmt.Printf("\nWrote #%d %s %s  Key: %s  Value: %s\n\n", rec.Seq, rec.Tapari, rec.Duna, rec.Key, rec.Value)
// 		return nil
//
// 	case "read", "r":
// 		return db.PrintAll() // served from memory, no WAL scan
//
// 	case "history", "h":
// 		return db.ReadHistory()
//
// 	default:
// 		return fmt.Errorf("unknown command %q (use write|read|history)", args[1])
// 	}
// }

// ---------- record ----------

type Record struct {
	Seq       uint64
	Tapari    string
	Duna      string
	Key       string
	Value     string
	Timestamp time.Time
	Hash      string
}

// group is a (Tapari, Duna) pair, used as a single map key.
type group struct{ Tapari, Duna string }

func sortedGroups[V any](m map[group]V) []group {
	gs := make([]group, 0, len(m))
	for g := range m {
		gs = append(gs, g)
	}
	sort.Slice(gs, func(i, j int) bool {
		if gs[i].Tapari != gs[j].Tapari {
			return gs[i].Tapari < gs[j].Tapari
		}
		return gs[i].Duna < gs[j].Duna
	})
	return gs
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// checksum covers every field of the record except the hash itself.
func checksum(seq uint64, tapari, duna, key, value string, nanos int64) string {
	h := fnv.New128a()
	h.Write([]byte(strconv.FormatUint(seq, 10) +
		valueSep + tapari + valueSep + duna + valueSep + key + valueSep + value + valueSep +
		strconv.FormatInt(nanos, 10)))
	return hex.EncodeToString(h.Sum(nil))
}

// sumHex hashes arbitrary bytes (used for the snapshot file).
func sumHex(b []byte) string {
	h := fnv.New128a()
	h.Write(b)
	return hex.EncodeToString(h.Sum(nil))
}

// ---------- write path ----------

// tapari/duna/key live in the space-separated header, so they must be
// non-empty, whitespace-free and must not contain ";;".
func validateHeaderField(name, s string) error {
	switch {
	case s == "":
		return fmt.Errorf("%s must not be empty", name)
	case strings.IndexFunc(s, unicode.IsSpace) >= 0:
		return fmt.Errorf("%s %q must not contain whitespace", name, s)
	case strings.Contains(s, valueSep):
		return fmt.Errorf("%s %q must not contain %q", name, s, valueSep)
	}
	return nil
}

func newRecord(seq uint64, tapari, duna, key, value string) (Record, error) {
	tapari, duna, key = strings.TrimSpace(tapari), strings.TrimSpace(duna), strings.TrimSpace(key)
	value = strings.TrimSpace(value)

	for _, f := range [...]struct{ name, val string }{
		{"tapari", tapari}, {"duna", duna}, {"key", key},
	} {
		if err := validateHeaderField(f.name, f.val); err != nil {
			return Record{}, err
		}
	}
	// The WAL is line-based, so a value can't contain line breaks.
	if strings.ContainsAny(value, "\r\n") {
		return Record{}, fmt.Errorf("value must not contain line breaks")
	}

	now := time.Now()
	return Record{
		Seq:       seq,
		Tapari:    tapari,
		Duna:      duna,
		Key:       key,
		Value:     value,
		Timestamp: now,
		Hash:      checksum(seq, tapari, duna, key, value, now.UnixNano()),
	}, nil
}

// encode: \n<seq> <tapari> <duna> <key> ;;<value>;; <unixnano> <hash>
func (r Record) encode() string {
	var b strings.Builder
	b.Grow(len(r.Tapari) + len(r.Duna) + len(r.Key) + len(r.Value) + len(r.Hash) + 64)
	b.WriteByte('\n')
	b.WriteString(strconv.FormatUint(r.Seq, 10))
	b.WriteByte(' ')
	b.WriteString(r.Tapari)
	b.WriteByte(' ')
	b.WriteString(r.Duna)
	b.WriteByte(' ')
	b.WriteString(r.Key)
	b.WriteString(" " + valueSep)
	b.WriteString(r.Value)
	b.WriteString(valueSep + " ")
	b.WriteString(strconv.FormatInt(r.Timestamp.UnixNano(), 10))
	b.WriteByte(' ')
	b.WriteString(r.Hash)
	return b.String()
}

// appendRecord writes one record in a single write call, then fsyncs.
func (wal *HoneyberriWAL) appendRecord(r Record) error {
	line := r.encode()

	wal.mu.Lock()
	defer wal.mu.Unlock()

	if _, err := wal.file.WriteString(line); err != nil {
		return err
	}
	return wal.file.Sync() // don't return until it's on disk
}

// ---------- read path ----------

// scanWAL streams the WAL top to bottom and calls fn for every valid record.
// Corrupt or partial lines are skipped and counted.
// It only scans: what to do with each record is up to the caller.
func (wal *HoneyberriWAL) scanWAL(fn func(Record)) (skipped int, err error) {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	if _, err := wal.file.Seek(0, io.SeekStart); err != nil {
		return 0, err
	}

	sc := bufio.NewScanner(wal.file)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)

	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		rec, ok := parseLine(line)
		if !ok {
			skipped++
			continue
		}
		fn(rec)
	}
	return skipped, sc.Err()
}

// parseLine parses exactly what encode writes:
//
//	<seq> <tapari> <duna> <key> ;;<value>;; <unixnano> <hash>
func parseLine(line string) (Record, bool) {
	open := strings.Index(line, valueSep)
	end := strings.LastIndex(line, valueSep)
	if open < 0 || end <= open {
		return Record{}, false
	}

	head := strings.Fields(line[:open])
	tail := strings.Fields(line[end+len(valueSep):])
	if len(head) != 4 || len(tail) != 2 {
		return Record{}, false
	}

	seq, err := strconv.ParseUint(head[0], 10, 64)
	if err != nil {
		return Record{}, false
	}
	nanos, err := strconv.ParseInt(tail[0], 10, 64)
	if err != nil {
		return Record{}, false
	}

	rec := Record{
		Seq:       seq,
		Tapari:    head[1],
		Duna:      head[2],
		Key:       head[3],
		Value:     line[open+len(valueSep) : end],
		Timestamp: time.Unix(0, nanos),
		Hash:      tail[1],
	}
	if checksum(rec.Seq, rec.Tapari, rec.Duna, rec.Key, rec.Value, nanos) != rec.Hash {
		return Record{}, false
	}
	return rec, true
}

func WarnSkipped(n int) {
	if n > 0 {
		fmt.Fprintf(os.Stderr, "\nskipped %d corrupt/partial line(s)\n", n)
	}
}

// ---------- history / h ----------

// ReadHistory prints every value ever written per key and saves it to HistoryName.
func (db *HoneyberriDB) ReadHistory() error {
	wal := db.wal
	h := make(map[group]map[string][]string)

	skipped, err := wal.scanWAL(func(r Record) {
		g := group{r.Tapari, r.Duna}
		m := h[g]
		if m == nil {
			m = make(map[string][]string)
			h[g] = m
		}
		m[r.Key] = append(m[r.Key], r.Value) // oldest first
	})
	if err != nil {
		return fmt.Errorf("failed to read WAL: %w", err)
	}
	if len(h) == 0 {
		fmt.Println("WAL is empty.")
		return nil
	}

	file, err := os.Create(HistoryName)
	if err != nil {
		return err
	}
	defer file.Close()

	// One render, two destinations: terminal and file.
	bw := bufio.NewWriter(io.MultiWriter(os.Stdout, file))
	writeHistory(bw, h)
	if err := bw.Flush(); err != nil {
		return err
	}

	WarnSkipped(skipped)
	return nil
}

// Format:
//
//	<Tapari> <Duna>
//	<key>:<value>,<value>,<value>
func writeHistory(w io.Writer, h map[group]map[string][]string) {
	for _, g := range sortedGroups(h) {
		fmt.Fprintf(w, "%s %s\n", g.Tapari, g.Duna)
		keys := h[g]
		for _, k := range sortedKeys(keys) {
			fmt.Fprintf(w, "%s:%s\n", k, strings.Join(keys[k], ","))
		}
		fmt.Fprintln(w)
	}
}

// ---------- in-memory DB ----------

// entry is what memory keeps per key: the value and when it was written.
type entry struct {
	Value string `json:"v"`
	Nanos int64  `json:"t"`
}

type layout map[string]map[string]map[string]entry // tapari -> duna -> key -> entry

// Lock order (always): db.mu -> wal.mu. Never take db.mu while holding wal.mu.
type HoneyberriDB struct {
	mu      sync.RWMutex
	layout  layout
	lastSeq uint64 // highest seq applied to memory
	snapSeq uint64 // seq covered by the snapshot on disk
	wal     *HoneyberriWAL

	snapMu sync.Mutex // serializes snapshot writers (they share one temp file)
}

// OpenDb rebuilds memory: load snapshot, then replay WAL records with seq > snapshot's seq.
func OpenDb(wal *HoneyberriWAL) (*HoneyberriDB, int, error) {
	db := &HoneyberriDB{layout: make(layout), wal: wal}

	db.mu.Lock()
	defer db.mu.Unlock()

	if err := db.loadSnapshotLocked(SnapshotName); err != nil {
		// A bad snapshot is not fatal: the WAL still has everything.
		fmt.Fprintf(os.Stderr, "ignoring snapshot (%v); replaying full WAL\n", err)
		db.layout, db.lastSeq, db.snapSeq = make(layout), 0, 0
	}

	// db.mu is already held, so the callback uses applyLocked (no locking of its own).
	skipped, err := wal.scanWAL(func(r Record) {
		if r.Seq <= db.lastSeq {
			return // already in the snapshot
		}
		db.applyLocked(r)
		db.lastSeq = r.Seq
	})
	if err != nil {
		return nil, skipped, fmt.Errorf("failed to replay WAL: %w", err)
	}
	return db, skipped, nil
}

func (db *HoneyberriDB) CloseDb() error {
	return db.DBsnapShot(SnapshotName)
}

// applyLocked updates memory; caller must hold db.mu for writing.
func (db *HoneyberriDB) applyLocked(r Record) {
	duna := db.layout[r.Tapari]
	if duna == nil {
		duna = make(map[string]map[string]entry)
		db.layout[r.Tapari] = duna
	}
	keys := duna[r.Duna]
	if keys == nil {
		keys = make(map[string]entry)
		duna[r.Duna] = keys
	}
	keys[r.Key] = entry{Value: r.Value, Nanos: r.Timestamp.UnixNano()}
}

// Set assigns the next seq, appends to the WAL (durable), then updates memory.
// All under one lock, so memory order always equals WAL order.
func (db *HoneyberriDB) Set(tapari, duna, key, value string) (Record, error) {
	db.mu.Lock()
	defer db.mu.Unlock()

	rec, err := newRecord(db.lastSeq+1, tapari, duna, key, value)
	if err != nil {
		return Record{}, err
	}
	if err := db.wal.appendRecord(rec); err != nil {
		return Record{}, fmt.Errorf("failed to write WAL: %w", err)
	}
	db.applyLocked(rec)
	db.lastSeq = rec.Seq
	return rec, nil
}

// Get reads straight from memory.
func (db *HoneyberriDB) Get(tapari, duna, key string) (string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()
	e, ok := db.layout[tapari][duna][key] // indexing nil maps is safe in Go
	return e.Value, ok
}

// PrintAll prints the current state from memory.
func (db *HoneyberriDB) PrintAll() error {
	var buf bytes.Buffer

	db.mu.RLock()
	for _, t := range sortedKeys(db.layout) {
		fmt.Fprintf(&buf, "\n%s\n", t)
		for _, d := range sortedKeys(db.layout[t]) {
			fmt.Fprintf(&buf, "├── %s\n", d)
			keys := db.layout[t][d]
			for _, k := range sortedKeys(keys) {
				e := keys[k]
				fmt.Fprintf(&buf, "│   └── %s --> %s  (%s)\n", k, e.Value,
					time.Unix(0, e.Nanos).UTC().Format("2006-01-02 15:04:05"))
			}
		}
	}
	empty := len(db.layout) == 0
	db.mu.RUnlock()

	if empty {
		fmt.Println("DB is empty.")
		return nil
	}
	buf.WriteByte('\n')
	_, err := os.Stdout.Write(buf.Bytes()) // print outside the lock
	return err
}

// ---------- snapshot ----------

// File layout: <json>\n<fnv128a hex of the json>
type snapshotFile struct {
	Seq    uint64 `json:"seq"` // the snapshot covers every WAL record with seq <= this
	Layout layout `json:"layout"`
}

// DBsnapShot writes the current state atomically (temp file -> fsync -> rename).
// It does nothing if nothing changed since the last snapshot.
func (db *HoneyberriDB) DBsnapShot(filename string) error {
	db.snapMu.Lock()
	defer db.snapMu.Unlock()

	// Hold the read lock only while encoding to bytes; disk I/O happens outside it.
	db.mu.RLock()
	seq := db.lastSeq
	if seq == db.snapSeq {
		db.mu.RUnlock()
		return nil
	}
	payload, err := json.Marshal(snapshotFile{Seq: seq, Layout: db.layout})
	db.mu.RUnlock()
	if err != nil {
		return fmt.Errorf("failed to encode snapshot: %w", err)
	}

	data := append(payload, '\n')
	data = append(data, sumHex(payload)...)

	tmp := filename + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("failed to create snapshot file: %w", err)
	}
	_, err = f.Write(data)
	if err == nil {
		err = f.Sync()
	}
	if cerr := f.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp, filename)
	}
	if err != nil {
		os.Remove(tmp)
		return fmt.Errorf("failed to write snapshot: %w", err)
	}
	syncDir(filepath.Dir(filename)) // make the rename itself durable (best effort)

	db.mu.Lock()
	if seq > db.snapSeq {
		db.snapSeq = seq
	}
	db.mu.Unlock()
	return nil
}

func syncDir(dir string) {
	if d, err := os.Open(dir); err == nil {
		d.Sync()
		d.Close()
	}
}

// loadSnapshotLocked restores state from the snapshot; caller must hold db.mu.
func (db *HoneyberriDB) loadSnapshotLocked(filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // fresh DB
		}
		return fmt.Errorf("failed to read snapshot: %w", err)
	}

	i := bytes.LastIndexByte(data, '\n')
	if i < 0 {
		return fmt.Errorf("snapshot is truncated")
	}
	payload, sum := data[:i], strings.TrimSpace(string(data[i+1:]))
	if sumHex(payload) != sum {
		return fmt.Errorf("snapshot checksum mismatch")
	}

	var s snapshotFile
	if err := json.Unmarshal(payload, &s); err != nil {
		return fmt.Errorf("failed to decode snapshot: %w", err)
	}
	if s.Layout == nil {
		s.Layout = make(layout)
	}
	db.layout, db.lastSeq, db.snapSeq = s.Layout, s.Seq, s.Seq
	return nil
}

// for server

// GetFullDump returns a copy of the entire database layout for HTTP/API use.
func (db *HoneyberriDB) GetFullDump() layout {
	db.mu.RLock()
	defer db.mu.RUnlock()

	// Deep copy to avoid race conditions outside the lock
	dump := make(layout)
	for t, dunas := range db.layout {
		dump[t] = make(map[string]map[string]entry)
		for d, keys := range dunas {
			dump[t][d] = make(map[string]entry)
			for k, e := range keys {
				dump[t][d][k] = e
			}
		}
	}
	return dump
}

// GetTapari returns all dunas and key-values for a specific Tapari.
func (db *HoneyberriDB) GetTapari(tapari string) (map[string]map[string]string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	dunas, ok := db.layout[tapari]
	if !ok {
		return nil, false
	}

	result := make(map[string]map[string]string)
	for duna, keys := range dunas {
		result[duna] = make(map[string]string)
		for k, e := range keys {
			result[duna][k] = e.Value
		}
	}
	return result, true
}

// GetDuna returns all key-value pairs for a specific Tapari and Duna.
func (db *HoneyberriDB) GetDuna(tapari, duna string) (map[string]string, bool) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	keys, ok := db.layout[tapari][duna]
	if !ok {
		return nil, false
	}

	result := make(map[string]string)
	for k, e := range keys {
		result[k] = e.Value
	}
	return result, true
}
