package main

import (
	"fmt"
	"net/http"
)

type apiConfig struct {
	fileserverHits int
}

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
	mux.HandleFunc("/healthz", handlerReadiness)
	mux.HandleFunc("/metrics", apiCfg.handlerFileServerHitsCounter)
	mux.HandleFunc("/reset", apiCfg.handlerResetFileServerHits)

	server := http.Server{
		Addr:    ":8080",
		Handler: middlewareCors(mux), // This is executed when *any* request is made to the server, while the handlers defined for each route only run when requests are made to that route
	}
	server.ListenAndServe()
}

func handlerReadiness(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusOK)                    // This is for the status code of the HTTP response
	w.Write([]byte(http.StatusText(http.StatusOK))) // This is used to write the response body, and must come after WriteHeader()

	return
}

func (cfg *apiConfig) handlerFileServerHitsCounter(w http.ResponseWriter, r *http.Request) {
	hits := fmt.Sprintf("Hits: %d", cfg.fileserverHits)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte(hits))
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
