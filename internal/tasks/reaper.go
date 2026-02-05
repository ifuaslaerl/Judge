package tasks

import (
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"github.com/ifuaslaerl/Judge/internal/data"
)

// StartReaper cleans up orphaned files in storage/submissions
func StartReaper() {
	log.Println("REAPER: Starting cleanup scan...")

	dir := filepath.Join("storage", "submissions")
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Printf("REAPER WARNING: Could not read storage directory: %v", err)
		return
	}

	deletedCount := 0
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == ".keep" {
			continue
		}

		// Filename format is assumed to be "{id}.cpp"
		filename := entry.Name()
		idStr := strings.TrimSuffix(filename, filepath.Ext(filename))
		id, err := strconv.Atoi(idStr)
		
		if err != nil {
			// If file doesn't follow strict {id}.cpp format, ignore or warn
			continue
		}

		// Check if ID exists in DB
		var exists bool
		err = data.DB.QueryRow("SELECT EXISTS(SELECT 1 FROM submissions WHERE id = ?)", id).Scan(&exists)
		if err != nil {
			log.Printf("REAPER DB ERROR: %v", err)
			continue
		}

		if !exists {
			fullPath := filepath.Join(dir, filename)
			if err := os.Remove(fullPath); err == nil {
				log.Printf("REAPER: Deleted orphan file %s", filename)
				deletedCount++
			}
		}
	}

	log.Printf("REAPER: Cleanup complete. Deleted %d orphans.", deletedCount)
}
