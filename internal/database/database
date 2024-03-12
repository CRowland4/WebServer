package database

import (
    "os"
    "sync"
    "github.com/CRowland4/WebServer/main"
)

type DB struct {
    path string
    mux *sync.RWMutex
}

type DBStructure struct {
    Chirps map[int]main.Chirp `json:"chirps"`
}

func NewDB(path string) (new_database *DB) {
    new_database = DB{
        path: "./database/database.json",
    }

    os.Create(new_database.path)
    return new_database
}

