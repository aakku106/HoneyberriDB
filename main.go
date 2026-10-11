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

		if _, err := file.WriteString("{" + Version + ":" + args[2] + "," + args[3] + "}"); err != nil {
			log.Fatalln(err)
		}

	case "read", "r":
		data, err := os.ReadFile(DbName)
		if err != nil {
			log.Fatalln(err)
		}

		fmt.Printf("data: %v\n", string(data))
	}
}
