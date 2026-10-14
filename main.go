package main

import (
	"bufio"
	"encoding/hex"
	"encoding/json"
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
)

const (
	SoftwareVersion string = "0.0.1-alpha.2"
	DBname          string = "HoneyberriDB"
	WALname         string = DBname + SoftwareVersion + ".aakku"
)

type HoneyberriWAL struct {
	mu       sync.RWMutex
	filename string
	file     *os.File
}

func OpenWAL() (*HoneyberriWAL, error) {
	// Open the WAL ant let it be opean
	file, err := os.OpenFile(WALname, os.O_APPEND|os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return &HoneyberriWAL{
		filename: WALname,
		file:     file,
	}, nil
}

type FinalData map[string]string

func main() {
	wal, err := OpenWAL()
	if err != nil {
		log.Fatalln(err)
	}
	defer wal.file.Close()

	args := os.Args
	if len(args) < 2 {
		log.Fatalln("Need to be more than 1 args to write,read,history,parc")
	}

	switch args[1] {
	case "write", "w":
		wal.writeWAL(args)
		fmt.Printf("\nSuccessfully wrote Key:\t%s,\tValue:\t%s\tto database!\n\n", args[2], args[3])

	case "read", "r":
		wal.ReadWAL()

	case "history", "h":
		wal.readAllHistory()

	case "parse", "p":
		wal.parcJSON("./data.json")

	default:
		fmt.Println("IndifinateBehavior")
	}
}

func (wal *HoneyberriWAL) writeWAL(args []string) {

	tapari := strings.TrimSpace(args[2])
	duna := strings.TrimSpace(args[3])
	key := strings.TrimSpace(args[4])
	value := strings.TrimSpace(args[5])

	payload := fmt.Sprintf(";;%s;;%s;;%s;;%s;;", tapari, duna, key, value)

	payloadHasher := fnv.New128a()
	_, err := io.WriteString(payloadHasher, payload)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to write to hasher: %v\n", err)
		os.Exit(1)
	}

	rawHash := payloadHasher.Sum(nil)
	checkSum := hex.EncodeToString(rawHash)

	timestamp := time.Now().UnixNano()

	record := fmt.Sprintf("%s %s %s ;;%s;; %d %s\n", tapari, duna, key, value, timestamp, checkSum)

	wal.mu.Lock()
	defer wal.mu.Unlock()
	if _, err := wal.file.WriteString(record); err != nil {
		log.Fatalln(err)
	}
}

type Record struct {
	Tapari    string
	Duna      string
	Key       string
	Value     string
	Timestamp time.Time
	Hash      string
}

const valueSep = ";;"

// ---------- shared WAL scanning ----------

// scanWAL walks the WAL top to bottom and calls fn for every valid record.
// Corrupt or partial lines are skipped and counted.
func (wal *HoneyberriWAL) scanWAL(fn func(Record)) (skipped int, err error) {
	wal.mu.Lock() // Lock, not RLock: Seek moves the shared file offset
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

// Line format (see writeWAL):
//
//	<tapari> <duna> <key> ;;<value>;; <unixnano|formatted time> <fnv128a hex>
func parseLine(line string) (Record, bool) {
	open := strings.Index(line, valueSep)
	end := strings.LastIndex(line, valueSep)
	if open < 0 || end <= open {
		return Record{}, false
	}

	head := strings.Fields(line[:open])
	if len(head) != 3 {
		return Record{}, false
	}
	tail := strings.Fields(line[end+len(valueSep):])
	if len(tail) < 2 {
		return Record{}, false
	}

	ts, ok := parseTimestamp(strings.Join(tail[:len(tail)-1], " "))
	if !ok {
		return Record{}, false
	}

	rec := Record{
		Tapari:    head[0],
		Duna:      head[1],
		Key:       head[2],
		Value:     line[open+len(valueSep) : end],
		Timestamp: ts,
		Hash:      tail[len(tail)-1],
	}

	// Integrity check: same payload string writeWAL hashed
	h := fnv.New128a()
	io.WriteString(h, fmt.Sprintf(";;%s;;%s;;%s;;%s;;", rec.Tapari, rec.Duna, rec.Key, rec.Value))
	if hex.EncodeToString(h.Sum(nil)) != rec.Hash {
		return Record{}, false
	}
	return rec, true
}

func parseTimestamp(s string) (time.Time, bool) {
	if n, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(0, n).UTC(), true
	}
	if t, err := time.Parse("2006-01-02 15:04:05.999999999 -0700 MST", s); err == nil {
		return t, true
	}
	return time.Time{}, false
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func warnSkipped(n int) {
	if n > 0 {
		fmt.Fprintf(os.Stderr, "\nskipped %d corrupt/partial line(s)\n", n)
	}
}

// ---------- read / r ----------

type state map[string]map[string]map[string]Record // tapari -> duna -> key -> latest

func (wal *HoneyberriWAL) ReadWAL() {
	st := make(state)

	skipped, err := wal.scanWAL(func(r Record) {
		if st[r.Tapari] == nil {
			st[r.Tapari] = make(map[string]map[string]Record)
		}
		if st[r.Tapari][r.Duna] == nil {
			st[r.Tapari][r.Duna] = make(map[string]Record)
		}
		st[r.Tapari][r.Duna][r.Key] = r // later line overwrites earlier
	})
	if err != nil {
		log.Fatalln("failed to read WAL:", err)
	}
	if len(st) == 0 {
		fmt.Println("WAL is empty.")
		return
	}

	displayDb(st)
	warnSkipped(skipped)
}

func displayDb(st state) {
	for _, tapari := range sortedKeys(st) {
		fmt.Printf("\n%s\n", tapari)
		for _, duna := range sortedKeys(st[tapari]) {
			fmt.Printf("├── %s\n", duna)
			for _, key := range sortedKeys(st[tapari][duna]) {
				r := st[tapari][duna][key]
				fmt.Printf("│   └── %s --> %s  (%s)\n", key, r.Value, r.Timestamp.Format("2006-01-02 15:04:05"))
			}
		}
	}
	fmt.Println()
}

// ---------- history / h ----------

type history map[string]map[string]map[string][]string // tapari -> duna -> key -> values (oldest first)

func (wal *HoneyberriWAL) readAllHistory() {
	h := make(history)

	skipped, err := wal.scanWAL(func(r Record) {
		if h[r.Tapari] == nil {
			h[r.Tapari] = make(map[string]map[string][]string)
		}
		if h[r.Tapari][r.Duna] == nil {
			h[r.Tapari][r.Duna] = make(map[string][]string)
		}
		h[r.Tapari][r.Duna][r.Key] = append(h[r.Tapari][r.Duna][r.Key], r.Value)
	})
	if err != nil {
		log.Fatalln("failed to read WAL:", err)
	}
	if len(h) == 0 {
		fmt.Println("WAL is empty.")
		return
	}

	// Print to terminal
	writeHistory(os.Stdout, h)

	// Save to file
	file, err := os.Create("History" + SoftwareVersion + ".aakku")
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	bw := bufio.NewWriter(file)
	writeHistory(bw, h)
	if err := bw.Flush(); err != nil {
		log.Fatalln(err)
	}

	warnSkipped(skipped)
}

// Format:
//
//	<Tapari> <Duna>
//	<key>:<value>,<value>,<value>
func writeHistory(w io.Writer, h history) {
	for _, tapari := range sortedKeys(h) {
		for _, duna := range sortedKeys(h[tapari]) {
			fmt.Fprintf(w, "%s %s\n", tapari, duna)
			for _, key := range sortedKeys(h[tapari][duna]) {
				fmt.Fprintf(w, "%s:%s\n", key, strings.Join(h[tapari][duna][key], ","))
			}
			fmt.Fprintln(w)
		}
	}
}
func (wal *HoneyberriWAL) parcJSON(path string) {
	fileContent, err := os.ReadFile(path)
	if err != nil {
		log.Fatalf("failed to read file: %s", err)
	}
	data := make(map[string]any)
	if err := json.Unmarshal(fileContent, &data); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("data: %v\n", data)
	args := make([]string, 4)
	for key, value := range data {
		args[2] = key
		args[3] = fmt.Sprintf("%v", value)
		wal.writeWAL(args)
	}
}
