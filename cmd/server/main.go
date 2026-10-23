package main

import (
	"fmt"
	h "honeyberridb"
	"log"
	"net/http"
)

func main() {
	fmt.Println("Opening DB")
	db, err := h.Open()
	if err != nil {
		log.Fatalln(err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /write", write)
	mux.HandleFunc("GET /readAll", readAll)
	server := http.Server{
		Addr:    ":106106",
		Handler: mux,
	}

	fmt.Println("Lunching Server")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}

func write(w http.ResponseWriter, r *http.Request)   {}
func readAll(w http.ResponseWriter, r *http.Request) {}
