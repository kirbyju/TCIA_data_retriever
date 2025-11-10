package main

import (
	"bufio"
	"os"
	"path/filepath"
	"sync"
)

// ProcessedFilesDB is a simple file-based database to track processed files.
type ProcessedFilesDB struct {
	path string
	mu   sync.Mutex
	uris map[string]struct{}
}

// NewProcessedFilesDB creates a new instance of ProcessedFilesDB.
func NewProcessedFilesDB(outputDir string) (*ProcessedFilesDB, error) {
	db := &ProcessedFilesDB{
		path: filepath.Join(outputDir, ".processed_files.log"),
		uris: make(map[string]struct{}),
	}
	if err := db.load(); err != nil {
		return nil, err
	}
	return db, nil
}

// load reads the database file into memory.
func (db *ProcessedFilesDB) load() error {
	db.mu.Lock()
	defer db.mu.Unlock()

	file, err := os.Open(db.path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // No database yet, that's fine.
		}
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		db.uris[scanner.Text()] = struct{}{}
	}
	return scanner.Err()
}

// Add adds a URI to the database and saves it to the file.
func (db *ProcessedFilesDB) Add(uri string) error {
	db.mu.Lock()
	defer db.mu.Unlock()

	// Add to in-memory map
	db.uris[uri] = struct{}{}

	// Append to file
	file, err := os.OpenFile(db.path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer file.Close()

	_, err = file.WriteString(uri + "\n")
	return err
}

// Contains checks if a URI is already in the database.
func (db *ProcessedFilesDB) Contains(uri string) bool {
	db.mu.Lock()
	defer db.mu.Unlock()

	_, exists := db.uris[uri]
	return exists
}
