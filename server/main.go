package main

import (
	"HoneyberriDB"
	"log"
	"net/http"
)

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /write", write)
	mux.HandleFunc("GET /readAll", readAll)
	server := http.Server{
		Addr:    ":106106",
		Handler: mux,
	}
	if err := server.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}

func Open() {
}

func write(w http.ResponseWriter, r *http.Request)   {}
func readAll(w http.ResponseWriter, r *http.Request) {}
