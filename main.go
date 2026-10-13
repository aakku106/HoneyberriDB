package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
)

const (
	SoftwareVersion string = "0.0.1-alpha.1"
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
		log.Fatalln("Need to be more than 3 to write and 3 ot read ")
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

var WriteWalLock sync.RWMutex

func (wal *HoneyberriWAL) writeWAL(args []string) {

	record := fmt.Sprintf("{%s:%s}", args[2], args[3])

	wal.mu.Lock()
	if _, err := wal.file.WriteString(record); err != nil {
		log.Fatalln(err)
	}
	wal.mu.Unlock()
}

func (wal *HoneyberriWAL) ReadWAL() {
	fileContent := wal.readAllWAL()

	records := extractRecords(string(fileContent))
	data := sanitizeRecords(records)
	displayDb(data)
}

func (wal *HoneyberriWAL) readAllWAL() []byte {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	// 1. Move the file cursor back to the very beginning (offset 0)
	_, err := wal.file.Seek(0, 0)
	if err != nil {
		log.Fatalln("failed to seek file:", err)
	}

	// 2. Get the file info to know how many bytes to read
	fileInfo, err := wal.file.Stat()
	if err != nil {
		log.Fatalln("failed to get file stats:", err)
	}

	// If the file is empty, handle it gracefully
	if fileInfo.Size() == 0 {
		fmt.Println("WAL is empty.")
		return nil
	}

	// 3. Read all the bytes from the open file handle into a buffer
	fileContent := make([]byte, fileInfo.Size())
	_, err = wal.file.Read(fileContent)
	if err != nil {
		log.Fatalln("failed to read WAL:", err)
	}

	return fileContent
}

// Removes curly brasses "{}" from all values in WAL and
// Returns slice of string []string
func extractRecords(fileContent string) []string {
	var records []string
	var current strings.Builder
	inBlock := false

	for _, ch := range fileContent {
		switch ch {
		case '{': // Yo { start bhako
			inBlock = true
			current.Reset()
			// current.WriteRune(ch)
		case '}': // Yo } ie end bhako
			if inBlock {
				// current.WriteRune(ch)
				records = append(records, current.String())
				inBlock = false
			}
		default: // yo baki sab j ni xa
			if inBlock {
				current.WriteRune(ch)
			}
		}
	}
	return records
}

// What Kind of data we get out of this func
/*
{0.0.1-alpha.0:Name,Adarasha Gaihre}
{0.0.1-alpha.0:Name,Adarasha Gaihre}
{0.0.1-alpha.0:age,20}
{0.0.1-alpha.0:Name,Aakku}
{0.0.1-alpha.0:suku,dead}
{0.0.1-alpha.0:suku,dead}
{0.0.1-alpha.0:fuku,dead}
{0.0.1-alpha.0:aakku,Very Good Boi}
*/

func sanitizeRecords(records []string) FinalData {
	data := make(map[string]string)

	for _, record := range records {
		// Cut splits the string at the *very first* occurrence of ":"
		key, value, found := strings.Cut(record, ":")

		if found {
			trimmedKey := strings.TrimSpace(key)
			trimmedVal := strings.TrimSpace(value)

			data[trimmedKey] = trimmedVal
		}
	}

	return data
}

func displayDb(data map[string]string) {
	for key, value := range data {
		if _, err := fmt.Printf("\n\n%s:\n└──", key); err != nil {
			log.Fatalln(err)
		}
		if _, err := fmt.Print(" --> " + value); err != nil {
			log.Fatalln(err)
		}
	}
}

func (wal *HoneyberriWAL) readAllHistory() {
	wal.mu.Lock()
	defer wal.mu.Unlock()

	_, err := wal.file.Seek(0, 0)
	if err != nil {
		log.Fatalln("failed to seek file:", err)
	}

	fileInfo, err := wal.file.Stat()
	if err != nil {
		log.Fatalln("failed to get file stats:", err)
	}

	if fileInfo.Size() == 0 {
		fmt.Println("WAL is empty.")
		return
	}

	fileContent := make([]byte, fileInfo.Size())
	_, err = wal.file.Read(fileContent)
	if err != nil {
		log.Fatalln("failed to read WAL:", err)
	}

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
