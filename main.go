package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang-jwt/jwt/v4"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/CRowland4/WebServer/internal/database"
	"github.com/CRowland4/WebServer/internal/httpStructs"
	"github.com/joho/godotenv"
)

type apiConfig struct {
	fileserverHits int
	jwtSecret      string
}

type userRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func main() {
	checkForDebugMode()
	_ = godotenv.Load(".env")
	apiCfg := apiConfig{
		fileserverHits: 0,
		jwtSecret:      os.Getenv("JWT_SECRET"),
	}
	mux := http.NewServeMux() // A "mux" or "multiplexer" is synonymous with "router"

	// This piece - http.StripPrefix("/app", http.FileServer(http.Dir("."))) - returns a http.Handler that serves up the index.html page.
	// The middleWareMetricsInc piece returns a http.Handler that just adds to the apiCfg fileServerHits count, and then executes the ServeHTTP(w, r) of the http.Handler
	//     that it's given - in this case, the one returned by the StripPrefix
	mux.Handle("/app/", apiCfg.middlewareMetricsInc(http.StripPrefix("/app", http.FileServer(http.Dir(".")))))
	// mux.HandleFunc takes a path, and then a handler function - the handler function just needs to have the signature <name>(http.ResponseWriter, *http.Request)
	// The mux.HandleFunc call handles the execution of the passed handler function when the given path is called
	mux.HandleFunc("/api/reset", apiCfg.handlerResetFileServerHits)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerFileServerHitsCounter)
	mux.HandleFunc("GET /api/chirps", handlerGetChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", handlerGetChirp)
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("POST /api/chirps", handlerPostChirps)
	mux.HandleFunc("POST /api/users", handlerPostUsers)
	mux.HandleFunc("POST /api/login", apiCfg.handlerPostLogin)
	mux.HandleFunc("PUT /api/users", handlerPutUsers)

	server := http.Server{
		Addr:    ":8080",
		Handler: middlewareCors(mux), // This is executed when *any* request is made to the server, while the handlers defined for each route only run when requests are made to that route
	}
	_ = server.ListenAndServe()
}

func handlerPutUsers(w http.ResponseWriter, r *http.Request) {
	var request httpStructs.PutUsersRequest
	decoder := json.NewDecoder(r.Body)
	errRequest := decoder.Decode(&request)
	if errRequest != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerPutUsers: Unable to decode")
		return
	}

	tokenString := r.Header.Get("Authorization")
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	claims := jwt.MapClaims{}
	token, errToken := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (any, error) {
		return os.Getenv("JWT_SECRET"), nil
	})
	if errToken != nil {
		msg := fmt.Sprintf("handlerPutUsers: %s: %s", errToken.Error(), tokenString)
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	claims, _ = token.Claims.(jwt.MapClaims)
	userID := claims["Subject"]
	database.UpdateUser(userID.(int), request)

	return
}

func (cfg *apiConfig) getJWT(request httpStructs.LoginRequest) (token string) {
	currentTime := time.Now().UTC()

	claims := jwt.RegisteredClaims{
		Issuer:    "chirpy",
		IssuedAt:  jwt.NewNumericDate(currentTime),
		ExpiresAt: jwt.NewNumericDate(currentTime.Add(time.Duration(request.ExpiresInSeconds))),
		Subject:   strconv.Itoa(database.GetUserByEmail(request.Email).ID),
	}
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	token, _ = newToken.SignedString(os.Getenv("JWT_SECRET"))
	fmt.Print(token)
	return token
}

func (cfg *apiConfig) handlerPostLogin(w http.ResponseWriter, r *http.Request) {
	// Receive and decode POST
	var request httpStructs.LoginRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&request)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerLogin: Unable to decode")
		return
	}

	if isMatch, user := database.UserPasswordMatch(request.Email, []byte(request.Password)); isMatch {
		response := httpStructs.LoginResponse{
			Email: user.Email,
			ID:    user.ID,
			Token: cfg.getJWT(request),
		}
		respondWithJson(w, http.StatusOK, response)
		return
	}

	respondWithError(w, http.StatusUnauthorized, "handlerLogin: Invalid password")
	return
}

func handlerPostUsers(w http.ResponseWriter, r *http.Request) {
	// Receive and decode POST
	var newUser userRequest
	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&newUser)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerPostUsers: Unable to decode")
		return
	}

	// Create and encode response
	user, err := database.GetUsersDatabase().CreateUser(newUser.Email, []byte(newUser.Password))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, err.Error())
		return
	}
	respondWithJson(w, http.StatusCreated, httpStructs.CreateNewUserResponse{Email: user.Email, ID: user.ID})

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

// checkForDebugMode removes the database files if the program is run with debug mode enabled
func checkForDebugMode() {
	debug := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	if *debug {
		_ = os.Remove("./internal/database/chirps.json")
		_ = os.Remove("./internal/database/users.json")
	}
}
