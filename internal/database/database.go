package database

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"slices"
	"strings"
	"sync"
)

type DB struct {
	path string
	mu  *sync.RWMutex
}

type Chirp struct {
	Body string `json:"body"`
	ID   int    `json:"id"`
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

	chirpsContent, _ := ioutil.ReadFile(db.path)
	json.Unmarshal(chirpsContent, &chirps)
	return chirps
}

func (db *DB) GetChirpByID(id int) (chirp Chirp, err error) {
	var chirps []Chirp

	chirpsContent, _ := ioutil.ReadFile(db.path)
	json.Unmarshal(chirpsContent, &chirps)

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
    ioutil.WriteFile(db.path, chirpsJSON, 0644)
    return
}

func GetDB() (db *DB) {
	database := DB{
		path: "./internal/database/database.json",
        mu: new(sync.RWMutex),
	}

    if _, err := os.Stat(database.path); err != nil {
        file, _ := os.Create(database.path)
        file.Close()
    }

	return &database
}

func cleanChirpBody(chirpBody string) (cleanedBody string) {
	words := strings.Split(chirpBody, " ")
	profaneWords := []string{"kerfuffle", "sharbert", "fornax"}

	cleanedWords := []string{}
	for _, word := range words {
		if slices.Contains(profaneWords, strings.ToLower(word)) {
			cleanedWords = append(cleanedWords, "****")
		} else {
			cleanedWords = append(cleanedWords, word)
		}
	}

	return strings.Join(cleanedWords, " ")
}
