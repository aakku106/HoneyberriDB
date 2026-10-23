package main

import (
	"fmt"
	h "honeyberridb"
	"log"
	"os"
	"strings"
)

func main() {
	if err := realMain(); err != nil {
		log.Fatalln(err)
	}
}
func realMain() error {

	db, err := h.Open()
	if err != nil {
		log.Fatalln(err)
	}
	defer db.Close()

	err = run(db, os.Args)
	return err
}

func run(db *h.HoneyberriDB, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: write|read|history")
	}

	switch args[1] {
	case "write", "w":
		if len(args) < 6 {
			return fmt.Errorf("usage: write <tapari> <duna> <key> <value...>")
		}
		rec, err := db.Set(args[2], args[3], args[4], strings.Join(args[5:], " "))
		if err != nil {
			return err
		}
		fmt.Printf("\nWrote #%d %s %s  Key: %s  Value: %s\n\n", rec.Seq, rec.Tapari, rec.Duna, rec.Key, rec.Value)
		return nil

	case "read", "r":
		return db.PrintAll() // served from memory, no WAL scan

	case "history", "h":
		return db.ReadHistory()

	default:
		return fmt.Errorf("unknown command %q (use write|read|history)", args[1])
	}
}
