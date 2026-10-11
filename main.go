package main

import (
	"fmt"
	"log"
	"os"
	"strings"
)

const (
	SoftwareVersion string = "0.0.1-alpha.0"
	DbName          string = "AakkusPersonalDb.aakku"
)

type FinalData map[string]string

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatalln("Need to be more than 3 to write and 3 ot read ")
	}
	switch args[1] {
	case "write", "w":
		writeDb(args)
		fmt.Printf("\nSuccessfully wrote Key:\t%s,\tValue:\t%s\tto database!\n\n", args[2], args[3])

	case "read", "r":
		readAllDb()
	case "history", "h":
		readAllHistory()
	default:
		fmt.Println("IndifinateBehavior")
	}
}

func writeDb(args []string) {
	file, err := os.OpenFile(DbName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()

	record := fmt.Sprintf("{%s:%s,%s}", SoftwareVersion, args[2], args[3])
	if _, err := file.WriteString(record); err != nil {
		log.Fatalln(err)
	}
}

func readAllDb() {
	fileContent, err := os.ReadFile(DbName)
	if err != nil {
		log.Fatalln(err)
	}

	records := extractRecords(string(fileContent))
	data := sanitizeRecords(records)
	fmt.Println(data)

}

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
		words := strings.FieldsFunc(record, func(r rune) bool {
			return r == ':' || r == ','
		})
		if words[0] != SoftwareVersion {
			continue
		}
		data[words[1]] = words[2]
	}
	return data
}

func readAllHistory() {
	fileContent, err := os.ReadFile(DbName)
	if err != nil {
		log.Fatalln(err)
	}

	records := extractRecords(string(fileContent))
	history := buildKeyHistory(records)
	displayHistory(history)
	extractHistory(history)
}

func buildKeyHistory(records []string) map[string][]string {
	history := make(map[string][]string)
	for _, record := range records {
		words := strings.FieldsFunc(record, func(r rune) bool {
			return r == ':' || r == ','
		})
		if words[0] != SoftwareVersion {
			continue
		}
		history[words[1]] = append(history[words[1]], words[2])
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
	file, err := os.Create("history.aakku")
	if err != nil {
		log.Fatalln(err)
	}
	defer file.Close()
	for key, value := range history {
		if _, err := file.WriteString("\n" + key + "\n"); err != nil {
			log.Fatalln(err)
		}
		for _, val := range value {
			if _, err := file.WriteString(" --> " + val); err != nil {
				log.Fatalln(err)
			}
		}
	}
}
