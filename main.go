package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/CRowland4/WebServer/internal/database"
)

type apiConfig struct {
	fileserverHits int
}

func main() {
	checkForDebugMode()

	apiCfg := apiConfig{
		fileserverHits: 0,
	}
	mux := http.NewServeMux() // A "mux" or "multiplexer" is synonymous with "router"

	// This piece - http.StripPrefix("/app", http.FileServer(http.Dir("."))) - returns a http.Handler that serves up the index.html page.
	// The middleWareMetricsInc piece returns a http.Handler that just adds to the apiCfg fileServerHits count, and then executes the ServeHTTP(w, r) of the http.Handler
	//     that it's given - in this case, the one returned by the StripPrefix
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	// mux.HandleFunc takes a path, and then a handler function - the handler function just needs to have the signature <name>(http.ResponseWriter, *http.Request)
	// The mux.HandleFunc call handles the execution of the passed handler function when the given path is called
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerFileServerHitsCounter)
	mux.HandleFunc("GET /api/chirps", handlerGetChirp)
	mux.HandleFunc("GET /api/chirps/{chirpID}", handlerGetChirps)
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("GET /api/reset", apiCfg.handlerResetFileServerHits)
	mux.HandleFunc("POST /api/chirps", handlerPostChirps)
	mux.HandleFunc("POST /api/users", handlerPostUsers)

	server := http.Server{
		Addr:    ":8080",
		Handler: middlewareCors(mux), // This is executed when *any* request is made to the server, while the handlers defined for each route only run when requests are made to that route
	}
	_ = server.ListenAndServe()
}

// checkForDebugMode removes the database files if the program is run with debug mode enabled
func checkForDebugMode() {
	debug := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	if *debug {
		_ = os.Remove("./internal/database/chirps.json")
		_ = os.Remove("./internal/database/users.json")
	}
}

func handlerPostUsers(w http.ResponseWriter, r *http.Request) {
	// Receive and decode POST
	var incomingUser database.User
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&incomingUser)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerPostUsers: Unable to decode")
		return
	}

	// Create and encode response
	userDatabase := database.GetUsersDatabase()
	user := userDatabase.CreateUser(incomingUser.Email)
	respondWithJson(w, http.StatusCreated, user)

	return
}

func handlerPostChirps(w http.ResponseWriter, r *http.Request) {
	// Receive and decode POST
	var incomingChirp database.Chirp
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&incomingChirp)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerPostChirps: Unable to decode")
		return
	}

	// Create and encode response
	if len(incomingChirp.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp too long")
	} else {
		chirpDatabase := database.GetChirpsDatabase()
		chirp := chirpDatabase.CreateChirp(incomingChirp.Body)
		respondWithJson(w, http.StatusCreated, chirp)
	}

	return
}

func handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	integerID, err := strconv.Atoi(r.PathValue("chirpID"))
	if err != nil {
		msg := fmt.Sprintf("Invalid id: %s", r.PathValue("chirpID"))
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}

	chirpDatabase := database.GetChirpsDatabase()
	chirp, err := chirpDatabase.GetChirpByID(integerID)
	if err != nil {
		respondWithError(w, http.StatusNotFound, err.Error())
		return
	}

	respondWithJson(w, http.StatusOK, chirp)
	return
}

func handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	chirpDatabase := database.GetChirpsDatabase()
	respondWithJson(w, http.StatusOK, chirpDatabase.GetChirps())
	return
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)                           // This is for the status code of the HTTP response
	_, _ = w.Write([]byte(http.StatusText(http.StatusOK))) // This is used to write the response body, and must come after WriteHeader()

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

	_, _ = w.Write([]byte(fmt.Sprintf(template, cfg.fileserverHits)))
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

func respondWithError(w http.ResponseWriter, code int, msg string) {
	w.WriteHeader(code)
	_, _ = w.Write([]byte(msg))
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
	_, _ = w.Write(jsonResponse)
	return
}
