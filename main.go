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

// ReadWAL replays the WAL top to bottom; the last write per db/bucket/key wins.
func (wal *HoneyberriWAL) ReadWAL() {
	state, skipped, err := wal.replay()
	if err != nil {
		log.Fatalln("failed to read WAL:", err)
	}
	if len(state) == 0 {
		fmt.Println("WAL is empty.")
		return
	}
	displayDb(state)
	if skipped > 0 {
		fmt.Fprintf(os.Stderr, "\nskipped %d corrupt/partial line(s)\n", skipped)
	}
}

type state map[string]map[string]map[string]Record // db -> bucket -> key -> latest

func (wal *HoneyberriWAL) replay() (state, int, error) {
	wal.mu.Lock() // Lock, not RLock: Seek moves the shared file offset
	defer wal.mu.Unlock()

	if _, err := wal.file.Seek(0, io.SeekStart); err != nil {
		return nil, 0, err
	}

	st := make(state)
	skipped := 0

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
		if st[rec.DB] == nil {
			st[rec.DB] = make(map[string]map[string]Record)
		}
		if st[rec.DB][rec.Bucket] == nil {
			st[rec.DB][rec.Bucket] = make(map[string]Record)
		}
		st[rec.DB][rec.Bucket][rec.Key] = rec // later line overwrites earlier
	}
	return st, skipped, sc.Err()
}

// Line format (see writeWAL):
//
//	<db> <bucket> <key> ;;<value>;; <unixnano|formatted time> <fnv128a hex>
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
		DB:        head[0],
		Bucket:    head[1],
		Key:       head[2],
		Value:     line[open+len(valueSep) : end],
		Timestamp: ts,
		Hash:      tail[len(tail)-1],
	}

	// Integrity check: same payload string writeWAL hashed
	h := fnv.New128a()
	io.WriteString(h, fmt.Sprintf(";;%s;;%s;;%s;;%s;;", rec.DB, rec.Bucket, rec.Key, rec.Value))
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

func displayDb(st state) {
	for _, db := range sortedKeys(st) {
		fmt.Printf("\n%s\n", db)
		for _, bucket := range sortedKeys(st[db]) {
			fmt.Printf("├── %s\n", bucket)
			for _, key := range sortedKeys(st[db][bucket]) {
				r := st[db][bucket][key]
				fmt.Printf("│   └── %s --> %s  (%s)\n", key, r.Value, r.Timestamp.Format("2006-01-02 15:04:05"))
			}
		}
	}
	fmt.Println()
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func (wal *HoneyberriWAL) readAllHistory() {
	fileContent := wal.readAllWAL()

	records := extractRecords(string(fileContent))
	history := buildKeyHistory(records)
	displayHistory(history)
	extractHistory(history)
}

func buildKeyHistory(records []string) map[string][]string {
	history := make(map[string][]string)

	for _, record := range records {
		// Cut splits the string at the *very first* occurrence of ":"
		key, value, found := strings.Cut(record, ":")

		if found {
			trimmedKey := strings.TrimSpace(key)
			trimmedVal := strings.TrimSpace(value)

			history[trimmedKey] = append(history[trimmedKey], trimmedVal)
		}
	}

	return history
}

func displayHistory(history map[string][]string) {
	for key, value := range history {
		if _, err := fmt.Printf("\n\n%s:\n└──", key); err != nil {
			log.Fatalln(err)
		}
		for _, v := range value {
			if _, err := fmt.Print(" --> " + v); err != nil {
				log.Fatalln(err)
			}
		}
	}
}

func extractHistory(history map[string][]string) {
	file, err := os.Create("History" + SoftwareVersion + ".aakku")
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	for key, value := range history {
		if _, err := file.WriteString("{" + key + ":"); err != nil {
			log.Fatalln(err)
		}
		for _, val := range value {
			if _, err := file.WriteString(val + ","); err != nil {
				log.Fatalln(err)
			}
		}
		if _, err := file.WriteString("}"); err != nil {
			log.Fatalln(err)
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
