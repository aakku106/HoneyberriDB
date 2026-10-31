package main

import (
	"encoding/json"
	"fmt"
	h "honeyberridb"
	"log"
	"net/http"
)

type App struct {
	db *h.HoneyberriDB
}

type WriteRequest struct {
	Tapari string `json:"tapari"`
	Duna   string `json:"duna"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}

func main() {
	fmt.Println("Opening DB")
	dbInstance, err := h.Open()
	if err != nil {
		log.Fatalln(err)
	}
	defer dbInstance.Close()

	app := &App{db: dbInstance}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /write", app.writeHandler)

	// The 4 read endpoints
	mux.HandleFunc("GET /readAll", app.readAllHandler)
	mux.HandleFunc("GET /tapari/{tapari}", app.readTapariHandler)
	mux.HandleFunc("GET /duna/{tapari}/{duna}", app.readDunaHandler)
	mux.HandleFunc("GET /key/{tapari}/{duna}/{key}", app.readKeyHandler)

	server := http.Server{
		Addr:    ":6106",
		Handler: mux,
	}

	fmt.Println("Launching Server on http://localhost:6106")
	if err := server.ListenAndServe(); err != nil {
		log.Fatalln(err)
	}
}

func (app *App) writeHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	var req WriteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	rec, err := app.db.Set(req.Tapari, req.Duna, req.Key, req.Value)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"status": "success", "seq": rec.Seq})
}

// 1. ReadAll: Spits out everything in DB as JSON
func (app *App) readAllHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	dump := app.db.GetFullDump()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(dump)
}

// 2. ReadTapari: Read everything inside a specific Tapari
func (app *App) readTapariHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	tapari := r.PathValue("tapari")
	data, found := app.db.GetTapari(tapari)
	if !found {
		http.Error(w, "Tapari not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// 3. ReadDuna: Read every key-value pair from a specific Duna
func (app *App) readDunaHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	tapari := r.PathValue("tapari")
	duna := r.PathValue("duna")
	data, found := app.db.GetDuna(tapari, duna)
	if !found {
		http.Error(w, "Duna or Tapari not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// 4. ReadKey: Just the value of that specific key
func (app *App) readKeyHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	tapari := r.PathValue("tapari")
	duna := r.PathValue("duna")
	key := r.PathValue("key")

	val, found := app.db.Get(tapari, duna, key)
	if !found {
		http.Error(w, "Key not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"tapari": tapari,
		"duna":   duna,
		"key":    key,
		"value":  val,
	})
}
