package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/CRowland4/WebServer/internal/httpStructs"
	"golang.org/x/crypto/bcrypt"
	"os"
	"slices"
	"strings"
	"sync"
	"time"
)

const (
	UsersDBPath         string = "./internal/database/users.json"
	ChirpsDBPath        string = "./internal/database/chirps.json"
	RevokedTokensDBPath string = "./internal/database/revoked_tokens.json"
)

type DB struct {
	path string
	mu   *sync.RWMutex
}

type Chirp struct {
	Body string `json:"body"`
	ID   int    `json:"id"`
}

type User struct {
	Email    string `json:"email"`
	ID       int    `json:"id"`
	Password []byte `json:"password"`
}

type RevokedToken struct {
	Time time.Time `json:"time"`
	ID   string    `json:"id"`
}

func (db *DB) CreateUser(email string, password []byte) (newUser User, err error) {
	users := db.GetUsers()
	if userAlreadyExists(email, users) {
		return newUser, errors.New(fmt.Sprintf("user %s already exists", email))
	}

	newUser = User{
		ID:       len(users) + 1,
		Email:    email,
		Password: []byte{}, // Added below
	}

	newUser.Password, _ = bcrypt.GenerateFromPassword(password, bcrypt.DefaultCost)
	users = append(users, newUser)
	db.SaveUsers(users)

	return newUser, nil
}

func (db *DB) CreateChirp(body string) (newChirp Chirp) {
	newChirp = Chirp{
		Body: cleanChirpBody(body),
	}

	chirps := db.GetChirps()
	newChirp.ID = len(chirps) + 1
	chirps = append(chirps, newChirp)
	db.SaveChirps(chirps)

	return newChirp
}

func (db *DB) GetChirpByID(id int) (chirp Chirp, err error) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	var chirps []Chirp
	chirpsContent, _ := os.ReadFile(db.path)
	_ = json.Unmarshal(chirpsContent, &chirps)

	for _, chirp := range chirps {
		if chirp.ID == id {
			return chirp, nil
		}
	}

	return chirp, errors.New(fmt.Sprintf("Cannot find chirp with ID %d", id))
}

func (db *DB) SaveChirps(chirps []Chirp) {
	db.mu.Lock()
	defer db.mu.Unlock()

	chirpsJSON, _ := json.Marshal(chirps)
	_ = os.WriteFile(db.path, chirpsJSON, 0644)
	return
}

func (db *DB) SaveUsers(users []User) {
	db.mu.Lock()
	defer db.mu.Unlock()

	usersJSON, _ := json.Marshal(users)
	_ = os.WriteFile(db.path, usersJSON, 0644)
	return
}

func cleanChirpBody(chirpBody string) (cleanedBody string) {
	words := strings.Split(chirpBody, " ")
	profaneWords := []string{"kerfuffle", "sharbert", "fornax"}

	var cleanedWords []string
	for _, word := range words {
		if slices.Contains(profaneWords, strings.ToLower(word)) {
			cleanedWords = append(cleanedWords, "****")
		} else {
			cleanedWords = append(cleanedWords, word)
		}
	}

	return strings.Join(cleanedWords, " ")
}

func userAlreadyExists(email string, users []User) bool {
	for _, user := range users {
		if user.Email == email {
			return true
		}
	}

	return false
}

func GetUserByEmail(email string) (user User) {
	for _, user := range GetDatabase(UsersDBPath).GetUsers() {
		if user.Email == email {
			return user
		}
	}

	return User{}
}

func UserPasswordMatch(email string, password []byte) (match bool, matchedUser User) {
	user := GetUserByEmail(email)
	if bcrypt.CompareHashAndPassword(user.Password, password) == nil {
		return true, user
	}

	return false, User{}
}

func RevokeToken(token string) {
	revokedTokensDatabase := GetDatabase(RevokedTokensDBPath)
	revokedTokens := revokedTokensDatabase.GetRevokedTokens()
	revokedToken := RevokedToken{
		Time: time.Now().UTC(),
		ID:   token,
	}

	revokedTokens = append(revokedTokens, revokedToken)
	revokedTokensDatabase.SaveRevokedTokens(revokedTokens)
	return
}

func (db *DB) SaveRevokedTokens(revokedTokens []RevokedToken) {
	db.mu.Lock()
	defer db.mu.Unlock()

	revokedTokensJSON, _ := json.Marshal(revokedTokens)
	_ = os.WriteFile(db.path, revokedTokensJSON, 0644)
	return
}

func UpdateUser(userID int, request httpStructs.UsersRequest) {
	userDB := GetDatabase(UsersDBPath)
	users := userDB.GetUsers()
	for i, user := range users {
		if user.ID == userID {
			newPassword, _ := bcrypt.GenerateFromPassword([]byte(request.Password), bcrypt.DefaultCost)

			updatedUser := User{
				ID:       userID,
				Email:    request.Email,
				Password: newPassword,
			}
			users[i] = updatedUser
			userDB.SaveUsers(users)
			break
		}
	}

	return
}

func (db *DB) GetChirps() (chirps []Chirp) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	chirpsContent, _ := os.ReadFile(db.path)
	_ = json.Unmarshal(chirpsContent, &chirps)
	return chirps
}

func (db *DB) GetUsers() (users []User) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	usersContent, _ := os.ReadFile(db.path)
	_ = json.Unmarshal(usersContent, &users)
	return users
}

func (db *DB) GetRevokedTokens() (revokedTokens []RevokedToken) {
	db.mu.RLock()
	defer db.mu.RUnlock()

	revokedTokensContent, _ := os.ReadFile(db.path)
	_ = json.Unmarshal(revokedTokensContent, &revokedTokens)
	return revokedTokens
}

func GetDatabase(dbPath string) (db *DB) {
	database := DB{
		path: dbPath,
		mu:   new(sync.RWMutex),
	}

	return &database
}

func CreateFreshDatabases() {
	_ = os.Remove(UsersDBPath)
	usersDB, _ := os.Create(UsersDBPath)
	_ = usersDB.Close()

	_ = os.Remove(ChirpsDBPath)
	chirpsDB, _ := os.Create(ChirpsDBPath)
	_ = chirpsDB.Close()

	_ = os.Remove(RevokedTokensDBPath)
	revokedTokensDB, _ := os.Create(RevokedTokensDBPath)
	_ = revokedTokensDB.Close()

	return
}
