package main

import (
	"log"
	"os"
	"path/filepath"
)

// WipeAll implements the "Weekly Wipe" maintenance logic
// 1. Deletes all files in storage/submissions
// 2. Wipes DB tables: submissions, sessions, users
func WipeAll() {
	log.Println("WARNING: STARTING WEEKLY WIPE. THIS IS DESTRUCTIVE.")

	// --- Step 1: Delete Files ---
	dir := filepath.Join("storage", "submissions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("WIPE ERROR: Could not read submissions directory: %v", err)
	}

	deletedCount := 0
	for _, entry := range entries {
		// Preserve the .keep file if it exists, delete everything else
		if entry.Name() == ".keep" {
			continue
		}

		fullPath := filepath.Join(dir, entry.Name())
		if err := os.Remove(fullPath); err != nil {
			log.Printf("WIPE WARNING: Failed to delete file %s: %v", entry.Name(), err)
		} else {
			deletedCount++
		}
	}
	log.Printf("DISK WIPE: Deleted %d submission files.", deletedCount)

	// --- Step 2: Wipe Database ---
	// We explicitly delete from child tables first, though CASCADE would handle it.
	// We DO NOT delete from 'problems', effectively resetting the contest
	// but keeping the problem set definition.
	queries := []string{
		"DELETE FROM submissions",
		"DELETE FROM sessions",
		"DELETE FROM users",
		"VACUUM", // Optional: Rebuild DB file to reclaim space
	}

	for _, q := range queries {
		if _, err := DB.Exec(q); err != nil {
			log.Fatalf("DB WIPE ERROR executing '%s': %v", q, err)
		}
	}

	log.Println("SUCCESS: System successfully wiped (Users, Sessions, Submissions).")
}
