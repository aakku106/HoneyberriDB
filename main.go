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
		fileContent, err := os.ReadFile(DbName)
		if err != nil {
			log.Fatalln(err)
		}

		records := extractRecords(string(fileContent))
		data := sanitizeRecords(records)
		fmt.Println(data)
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

func extractRecords(fileContent string) []string {
	var records []string
	var current strings.Builder
	inBlock := false

	for _, ch := range fileContent {
		switch ch {
		case '{': // Yo { start bhako
			inBlock = true
			current.Reset()
			current.WriteRune(ch)
		case '}': // Yo } ie end bhako
			if inBlock {
				current.WriteRune(ch)
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
		data[words[1]] = strings.TrimSuffix(words[2], "}")
	}
	return data
}
