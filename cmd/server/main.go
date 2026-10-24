package main

import (
	"fmt"
	h "honeyberridb"
	"log"
	"net/http"
)

type db struct {
	db *h.HoneyberriDB
}

func main() {
	fmt.Println("Opening DB")
	dbInstance, err := h.Open()
	if err != nil {
		log.Fatalln(err)
	}

	app := &db{db: dbInstance}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /write", app.write)
	mux.HandleFunc("GET /readAll", app.readAll)
	server := http.Server{
		Addr:    ":6106",
		Handler: mux,
	}

	fmt.Println("Lunching Server on port localhost:6106")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}

func (db *db) write(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Write endpoint reached"))
}
func (db db) readAll(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("ReadAll endpoint reached"))
}
