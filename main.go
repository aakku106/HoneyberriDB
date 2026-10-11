package main

import (
	"fmt"
	"log"
	"os"
)

const (
	Version string = "0.0.1-alpha.0"
	DbName  string = "AakkusPersonalDb.aakku"
)

var FinalData []map[string]string

func main() {
	args := os.Args
	if len(args) < 2 {
		log.Fatalln("Need to be more than 3 to write and 3 ot read ")
	}
	switch args[1] {
	case "write", "w":
		file, err := os.OpenFile(DbName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			log.Fatalln(err)
		}
		defer file.Close()

		record := fmt.Sprintf("{%s:%s,%s}", Version, args[2], args[3])
		if _, err := file.WriteString(record); err != nil {
			log.Fatalln(err)
		}
		fmt.Printf("\nSuccessfully wrote record:%s to database!\n\n", record)

	case "read", "r":
		data, err := os.ReadFile(DbName)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Printf("data: %v\n", string(data))
	}
}
