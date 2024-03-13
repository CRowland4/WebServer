package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"golang.org/x/crypto/bcrypt"
	"os"
	"slices"
	"strings"
	"sync"
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

func (db *DB) CreateUser(email string, password []byte) (newUser User, err error) {
	users := db.GetUsers()
	if userAlreadyExists(email, users) {
		return newUser, errors.New(fmt.Sprintf("user %s already exists", email))
	}

	newUser = User{
		Email: email,
	}

	newUser.ID = len(users) + 1
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

func GetChirpsDatabase() (db *DB) {
	database := DB{
		path: "./internal/database/chirps.json",
		mu:   new(sync.RWMutex),
	}

	if _, err := os.Stat(database.path); err != nil {
		file, _ := os.Create(database.path)
		_ = file.Close()
	}

	return &database
}

func GetUsersDatabase() (db *DB) {
	database := DB{
		path: "./internal/database/users.json",
		mu:   new(sync.RWMutex),
	}

	if _, err := os.Stat(database.path); err != nil {
		file, _ := os.Create(database.path)
		_ = file.Close()
	}

	return &database
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
	for _, user := range GetUsersDatabase().GetUsers() {
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
