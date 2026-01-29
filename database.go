package main

import (
	"database/sql"
	"log"
	"os"
	"time"

	_ "modernc.org/sqlite" // Pure Go SQLite driver
)

// DB is the global database connection pool
var DB *sql.DB

func InitDB() {
	var err error

	// 0. Verify Directory Exists
	if _, err := os.Stat("./storage/db"); os.IsNotExist(err) {
		log.Fatal("ERROR: Directory './storage/db' does not exist. Please run: mkdir -p storage/db")
	}

	log.Println("Initializing Database...")

	// 1. Open the database file
	// Note: The driver name is "sqlite", not "sqlite3" for modernc
	DB, err = sql.Open("sqlite", "./storage/db/judge.sqlite")
	if err != nil {
		log.Fatalf("Failed to open database struct: %v", err)
	}

	// 2. Verify connection (Ping)
	// We verify connection immediately to catch file creation errors
	if err = DB.Ping(); err != nil {
		log.Fatalf("Failed to ping database (Is the path correct?): %v", err)
	}

	// 3. Enforce WAL Mode & Timeout
	// We use a small delay to ensure the file is ready
	time.Sleep(100 * time.Millisecond)
	
	if _, err := DB.Exec("PRAGMA journal_mode=WAL;"); err != nil {
		log.Fatalf("Failed to enable WAL mode: %v", err)
	}
	if _, err := DB.Exec("PRAGMA busy_timeout=5000;"); err != nil {
		log.Fatalf("Failed to set busy timeout: %v", err)
	}

	// 4. Create Tables
	createSchema()

	log.Println("SUCCESS: Database connection initialized & Schema migrated.")
}

func createSchema() {
	schema := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT NOT NULL UNIQUE,
		password_hash TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS sessions (
		token TEXT PRIMARY KEY,
		user_id INTEGER NOT NULL,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS problems (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		letter_code TEXT NOT NULL,
		time_limit INTEGER NOT NULL,
		pdf_path TEXT NOT NULL
	);

	CREATE TABLE IF NOT EXISTS submissions (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		user_id INTEGER NOT NULL,
		problem_id INTEGER NOT NULL,
		status TEXT NOT NULL,
		file_path TEXT NOT NULL,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE,
		FOREIGN KEY(problem_id) REFERENCES problems(id) ON DELETE CASCADE
	);
	`

	_, err := DB.Exec(schema)
	if err != nil {
		log.Fatalf("Failed to migrate schema: %v", err)
	}
}
