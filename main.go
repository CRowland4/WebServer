package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"slices"
	"github.com/CRowland4/WebServer/database"
)

type apiConfig struct {
	fileserverHits int
}

type Chirp struct {
	Body string `json:"body"`
	ID int `json:"id"`
}

var chirpID = 0

func main() {
	apiCfg := apiConfig{
		fileserverHits: 0,
	}
	mux := http.NewServeMux() // A "mux" or "multiplexer" is synonymous with "router"

	// This piece - http.StripPrefix("/app", http.FileServer(http.Dir("."))) - returns a http.Handler that serves up the index.html page.
	// The middlewReMetricsInc piece returns a http.Handler that just adds to the apiCfg fileServerHits count, and then executes the ServeHTTP(w, r) of the http.Handler
	//     that it's given - in this case, the one returned by the StripPrefix
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	// mux.HandleFunc takes a path, and then a handler function - the handler function just needs to have the signature <name>(http.ResponseWriter, *http.Request)
	// The mux.HandleFunc call handles the execution of the passed handler function when the given path is called
	mux.HandleFunc("/api/reset", apiCfg.handlerResetFileServerHits)
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerFileServerHitsCounter)
	mux.HandleFunc("POST /api/chirps", handlerPostChirps)
	mux.HandleFunc("GET /api/chirps", handlerGetChirps)

	server := http.Server{
		Addr:    ":8080",
		Handler: middlewareCors(mux), // This is executed when *any* request is made to the server, while the handlers defined for each route only run when requests are made to that route
	}
	server.ListenAndServe()
}

func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	w.Write([]byte(msg))
	return
}

func respondWithJson(w http.ResponseWriter, code int, payload any) {
	jsonResponse, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(jsonResponse)
	return
}

func handlerPostChirps(w http.ResponseWriter, r *http.Request) {
	current_chirp := Chirp{}

	// Receive and decode POST
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&current_chirp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// Create and encode response
	var code int
	if len(current_chirp.Body) > 140 {
		code = http.StatusBadRequest
	} else {
		current_chirp.Body = cleanChirp(current_chirp.Body)
		chirpID++
		current_chirp.ID = chirpID
		code = http.StatusCreated
		saveChirpToDisk(current_chirp)
	}

	respondWithJson(w, code, current_chirp)
	return
}

func saveChirpToDisk(current_chirp Chirp) {

}

func cleanChirp(chirp string) (cleaned_chirp string) {
	words := strings.Split(chirp, " ")
	profaneWords := []string{"kerfuffle", "sharbert", "fornax"}
	
	cleaned_words := []string{}
	for _, word := range words {
		if slices.Contains(profaneWords, strings.ToLower(word)) {
			cleaned_words = append(cleaned_words, "****")
		} else {
			cleaned_words = append(cleaned_words, word)
		}
	}

	return strings.Join(cleaned_words, " ")
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)                    // This is for the status code of the HTTP response
	w.Write([]byte(http.StatusText(http.StatusOK))) // This is used to write the response body, and must come after WriteHeader()

	return
}

func (cfg *apiConfig) handlerFileServerHitsCounter(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	template := `<html>

	<body>
		<h1>Welcome, Chirpy Admin</h1>
		<p>Chirpy has been visited %d times!</p>
	</body>
	
	</html>`

	w.Write([]byte(fmt.Sprintf(template, cfg.fileserverHits)))
	return
}

func (cfg *apiConfig) middlewareMetricsInc(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cfg.fileserverHits++
		w.Header().Set("Cache-Control", "no-cache")
		next.ServeHTTP(w, r)
	})
}


func (cfg *apiConfig) handlerResetFileServerHits(w http.ResponseWriter, r *http.Request) {
	cfg.fileserverHits = 0
	return
}

func middlewareCors(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS, PUT, DELETE")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}
