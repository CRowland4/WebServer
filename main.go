package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/golang-jwt/jwt/v5"
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
}

func main() {
	checkForDebugMode()
	err := godotenv.Load()
	if err != nil {
		fmt.Println(err)
		return
	}
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
	mux.HandleFunc("/api/reset", apiCfg.handlerResetFileServerHits)
	mux.HandleFunc("GET /admin/metrics", apiCfg.handlerFileServerHitsCounter)
	mux.HandleFunc("GET /api/chirps", handlerGetChirps)
	mux.HandleFunc("GET /api/chirps/{chirpID}", handlerGetChirp)
	mux.HandleFunc("GET /api/healthz", handlerReadiness)
	mux.HandleFunc("POST /api/chirps", handlerPostChirps)
	mux.HandleFunc("POST /api/users", handlerPostUsers)
	mux.HandleFunc("POST /api/login", handlerPostLogin)
	mux.HandleFunc("PUT /api/users", handlerPutUsers)
	mux.HandleFunc("POST /api/revoke", handlerPostRevoke)
	// TODO PUT /API/USERS
	// TODO POST /API/REFRESH

	server := http.Server{
		Addr:    ":8080",
		Handler: middlewareCors(mux), // This is executed when *any* request is made to the server, while the handlers defined for each route only run when requests are made to that route
	}
	_ = server.ListenAndServe()
}

func handlerPostRevoke(w http.ResponseWriter, r *http.Request) {
	tokenString := r.Header.Get("Authorization")
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	database.RevokeToken(tokenString)
	w.WriteHeader(http.StatusOK)
	return
}

func getJWT(request httpStructs.LoginRequest, issuer string) (token string) {
	var ExpireTime time.Duration
	if issuer == "chirpy-access" {
		ExpireTime = time.Duration(1) * time.Hour
	} else {
		ExpireTime = time.Duration(60) * time.Hour * 24
	}

	claims := jwt.RegisteredClaims{
		Issuer:    issuer,
		IssuedAt:  jwt.NewNumericDate(time.Now().UTC()),
		ExpiresAt: jwt.NewNumericDate(time.Now().UTC().Add(ExpireTime)),
		Subject:   strconv.Itoa(database.GetUserByEmail(request.Email).ID),
	}
	newToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	token, _ = newToken.SignedString([]byte(os.Getenv("JWT_SECRET")))
	return token
}

func handlerPutUsers(w http.ResponseWriter, r *http.Request) {
	request, errRequest := decodeRequestBody(r, httpStructs.UsersRequest{})
	if errRequest != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerPutUsers: Unable to decode")
		return
	}

	tokenString := r.Header.Get("Authorization")
	tokenString = strings.TrimPrefix(tokenString, "Bearer ")

	claims := jwt.RegisteredClaims{}
	token, errToken := jwt.ParseWithClaims(
		tokenString,
		&claims,
		func(token *jwt.Token) (interface{}, error) { return []byte(os.Getenv("JWT_SECRET")), nil },
	)
	if errToken != nil {
		msg := fmt.Sprintf("handlerPutUsers: %s: %s", errToken.Error(), tokenString)
		respondWithError(w, http.StatusUnauthorized, msg)
		return
	}

	userID, _ := token.Claims.GetSubject()
	tokenIssuer, _ := token.Claims.GetIssuer()
	if tokenIssuer == "chirpy-refresh" {
		respondWithError(w, http.StatusUnauthorized, "handlerPutUsers: Refresh token received, need access token")
		return
	}
	userIDint, _ := strconv.Atoi(userID)
	database.UpdateUser(userIDint, request)

	response := httpStructs.UserUpdateResponse{
		Email: request.Email,
		ID:    userIDint,
	}
	respondWithJson(w, http.StatusOK, response)
	return
}

func handlerPostLogin(w http.ResponseWriter, r *http.Request) {
	request, err := decodeRequestBody(r, httpStructs.LoginRequest{})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerLogin: Unable to decode")
		return
	}

	if isMatch, user := database.UserPasswordMatch(request.Email, []byte(request.Password)); isMatch {
		response := httpStructs.LoginResponse{
			Email:        user.Email,
			ID:           user.ID,
			Token:        getJWT(request, "chirpy-access"),
			RefreshToken: getJWT(request, "chirpy-refresh"),
		}
		respondWithJson(w, http.StatusOK, response)
		return
	}

	respondWithError(w, http.StatusUnauthorized, "handlerLogin: Invalid password")
	return
}

func handlerPostUsers(w http.ResponseWriter, r *http.Request) {
	newUser, decodeErr := decodeRequestBody(r, httpStructs.UsersRequest{})
	if decodeErr != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerPostUsers: Unable to decode")
		return
	}

	user, createErr := database.GetDatabase(database.UsersDBPath).CreateUser(newUser.Email, []byte(newUser.Password))
	if createErr != nil {
		respondWithError(w, http.StatusBadRequest, createErr.Error())
		return
	}
	respondWithJson(w, http.StatusCreated, httpStructs.CreateNewUserResponse{Email: user.Email, ID: user.ID})

	return
}

func handlerPostChirps(w http.ResponseWriter, r *http.Request) {
	incomingChirp, err := decodeRequestBody(r, database.Chirp{})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "handlerPostChirps: Unable to decode")
		return
	}

	if len(incomingChirp.Body) > 140 {
		respondWithError(w, http.StatusBadRequest, "Chirp too long")
	} else {
		chirp := database.GetDatabase(database.ChirpsDBPath).CreateChirp(incomingChirp.Body)
		respondWithJson(w, http.StatusCreated, chirp)
	}

	return
}

func handlerGetChirp(w http.ResponseWriter, r *http.Request) {
	integerID, chirpIDErr := strconv.Atoi(r.PathValue("chirpID"))
	if chirpIDErr != nil {
		msg := fmt.Sprintf("Invalid id: %s", r.PathValue("chirpID"))
		respondWithError(w, http.StatusInternalServerError, msg)
		return
	}

	chirp, retrieveChirpErr := database.GetDatabase(database.ChirpsDBPath).GetChirpByID(integerID)
	if retrieveChirpErr != nil {
		respondWithError(w, http.StatusNotFound, retrieveChirpErr.Error())
		return
	}

	respondWithJson(w, http.StatusOK, chirp)
	return
}

func handlerGetChirps(w http.ResponseWriter, r *http.Request) {
	chirpDatabase := database.GetDatabase(database.UsersDBPath)
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

func decodeRequestBody[T any](r *http.Request, requestStruct T) (T, error) {
	decoder := json.NewDecoder(r.Body)
	errRequest := decoder.Decode(&requestStruct)
	return requestStruct, errRequest
}

// checkForDebugMode removes the database files if the program is run with debug mode enabled
func checkForDebugMode() {
	debug := flag.Bool("debug", false, "Enable debug mode")
	flag.Parse()

	if *debug {
		database.CreateFreshDatabases()
	}
	return
}
