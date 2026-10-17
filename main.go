package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

const (
	DBname          = "HoneyberriDB"
	SoftwareVersion = "0.0.1-alpha.2"
	WALversion      = "0.0.1-alpha.2"
	WALname         = DBname + WALversion + ".aakku"
	HistoryName     = "History." + DBname + WALversion + ".aakku"

	valueSep = ";;"
)

// ---------- WAL ----------

type HoneyberriWAL struct {
	mu   sync.Mutex // Mutex, not RWMutex: even reads Seek the shared file offset
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

func main() {
	wal, err := OpenWAL()
	if err != nil {
		log.Fatalln(err)
	}
	defer wal.Close()

	if err := run(wal, os.Args); err != nil {
		wal.Close() // log.Fatalln skips defers
		log.Fatalln(err)
	}
}

func run(wal *HoneyberriWAL, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: write|read|history")
	}

	switch args[1] {
	case "write", "w":
		if len(args) < 6 {
			return fmt.Errorf("usage: write <tapari> <duna> <key> <value...>")
		}
		rec, err := newRecord(args[2], args[3], args[4], strings.Join(args[5:], " "))
		if err != nil {
			return err
		}
		if err := wal.appendRecord(rec); err != nil {
			return fmt.Errorf("failed to write WAL: %w", err)
		}
		fmt.Printf("\nWrote %s %s  Key: %s  Value: %s\n\n", rec.Tapari, rec.Duna, rec.Key, rec.Value)
		return nil

	case "read", "r":
		return wal.ReadWAL()

	case "history", "h":
		return wal.ReadHistory()

	default:
		return fmt.Errorf("unknown command %q (use write|read|history)", args[1])
	}
}

// ---------- record ----------

type Record struct {
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
func checksum(tapari, duna, key, value string, nanos int64) string {
	h := fnv.New128a()
	h.Write([]byte(valueSep + tapari + valueSep + duna + valueSep + key + valueSep + value + valueSep))
	h.Write(strconv.AppendInt(nil, nanos, 10))
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

func newRecord(tapari, duna, key, value string) (Record, error) {
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
		Tapari:    tapari,
		Duna:      duna,
		Key:       key,
		Value:     value,
		Timestamp: now,
		Hash:      checksum(tapari, duna, key, value, now.UnixNano()),
	}, nil
}

// encode: \n<tapari> <duna> <key> ;;<value>;; <unixnano> <hash>
func (r Record) encode() string {
	var b strings.Builder
	b.Grow(len(r.Tapari) + len(r.Duna) + len(r.Key) + len(r.Value) + len(r.Hash) + 48)
	b.WriteByte('\n')
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
//	<tapari> <duna> <key> ;;<value>;; <unixnano> <hash>
func parseLine(line string) (Record, bool) {
	open := strings.Index(line, valueSep)
	end := strings.LastIndex(line, valueSep)
	if open < 0 || end <= open {
		return Record{}, false
	}

	head := strings.Fields(line[:open])
	tail := strings.Fields(line[end+len(valueSep):])
	if len(head) != 3 || len(tail) != 2 {
		return Record{}, false
	}

	nanos, err := strconv.ParseInt(tail[0], 10, 64)
	if err != nil {
		return Record{}, false
	}

	rec := Record{
		Tapari:    head[0],
		Duna:      head[1],
		Key:       head[2],
		Value:     line[open+len(valueSep) : end],
		Timestamp: time.Unix(0, nanos),
		Hash:      tail[1],
	}
	if checksum(rec.Tapari, rec.Duna, rec.Key, rec.Value, nanos) != rec.Hash {
		return Record{}, false
	}
	return rec, true
}

func warnSkipped(n int) {
	if n > 0 {
		fmt.Fprintf(os.Stderr, "\nskipped %d corrupt/partial line(s)\n", n)
	}
}

// ---------- read / r ----------

// ReadWAL replays the WAL; the last write per tapari/duna/key wins.
func (wal *HoneyberriWAL) ReadWAL() error {
	st := make(map[group]map[string]Record)

	skipped, err := wal.scanWAL(func(r Record) {
		g := group{r.Tapari, r.Duna}
		m := st[g]
		if m == nil {
			m = make(map[string]Record)
			st[g] = m
		}
		m[r.Key] = r // later line overwrites earlier
	})
	if err != nil {
		return fmt.Errorf("failed to read WAL: %w", err)
	}
	if len(st) == 0 {
		fmt.Println("WAL is empty.")
		return nil
	}

	out := bufio.NewWriter(os.Stdout)
	lastTapari := ""
	for _, g := range sortedGroups(st) {
		if g.Tapari != lastTapari {
			fmt.Fprintf(out, "\n%s\n", g.Tapari)
			lastTapari = g.Tapari
		}
		fmt.Fprintf(out, "├── %s\n", g.Duna)
		keys := st[g]
		for _, k := range sortedKeys(keys) {
			r := keys[k]
			fmt.Fprintf(out, "│   └── %s --> %s  (%s)\n", k, r.Value, r.Timestamp.UTC().Format("2006-01-02 15:04:05"))
		}
	}
	fmt.Fprintln(out)
	if err := out.Flush(); err != nil {
		return err
	}

	warnSkipped(skipped)
	return nil
}

// ---------- history / h ----------

// ReadHistory prints every value ever written per key and saves it to HistoryName.
func (wal *HoneyberriWAL) ReadHistory() error {
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

	warnSkipped(skipped)
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
