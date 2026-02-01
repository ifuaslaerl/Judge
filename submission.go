package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// HandleSubmission processes the upload: POST /submit/[problem_id]
func HandleSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 1. Get UserID from Context (set by Middleware)
	userID, ok := r.Context().Value(UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	// --- FIX #1: URL Parsing Fragility ---
	// Trim trailing slash to prevent empty string error on split
	cleanPath := strings.TrimSuffix(r.URL.Path, "/")
	parts := strings.Split(cleanPath, "/")
	
	if len(parts) == 0 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	// Takes the last part of the path as ID
	problemID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		http.Error(w, "Invalid Problem ID", http.StatusBadRequest)
		return
	}

	// 3. Enforce Submission Cap (Max 100 per user)
	var count int
	err = DB.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		log.Printf("DB Error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if count >= 100 {
		http.Error(w, "Submission limit reached (Max 100). Contact Admin.", http.StatusForbidden)
		return
	}

	// --- FIX #2: Upload Limit Miscalculation ---
	// Allow 1MB for file + 4KB overhead for Multipart headers/boundaries
	const maxUploadSize = (1024 * 1024) + 4096 
	
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large (Max 1MB)", http.StatusBadRequest)
		return
	}

	file, _, err := r.FormFile("code")
	if err != nil {
		http.Error(w, "Failed to retrieve 'code' file", http.StatusBadRequest)
		return
	}
	defer file.Close()

	// 5. Insert PENDING record into DB *FIRST*
	res, err := DB.Exec(`INSERT INTO submissions (user_id, problem_id, status, file_path) VALUES (?, ?, 'PENDING', '')`, userID, problemID)
	if err != nil {
		log.Printf("DB Insert Failed: %v", err)
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	
	submissionID, err := res.LastInsertId()
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}

	// 6. Write to Disk
	filename := fmt.Sprintf("%d.cpp", submissionID)
	filePath := filepath.Join("storage", "submissions", filename)

	dst, err := os.Create(filePath)
	if err != nil {
		// ROLLBACK: Delete DB record if file creation fails
		log.Printf("Disk Write Error: %v. Rolling back submission %d", err, submissionID)
		DB.Exec("DELETE FROM submissions WHERE id = ?", submissionID)
		http.Error(w, "Storage failure", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		// ROLLBACK: Delete DB record and partial file
		log.Printf("File Copy Error: %v. Rolling back submission %d", err, submissionID)
		os.Remove(filePath)
		DB.Exec("DELETE FROM submissions WHERE id = ?", submissionID)
		http.Error(w, "Storage failure", http.StatusInternalServerError)
		return
	}
	dst.Close()

	// --- FIX #3: Ignored Database Update Error ---
	// Check error on update. If DB lock fails here, we must rollback everything.
	if _, err := DB.Exec("UPDATE submissions SET file_path = ? WHERE id = ?", filePath, submissionID); err != nil {
		log.Printf("CRITICAL: Failed to link file path. Rolling back submission %d. Error: %v", submissionID, err)
		
		// Rollback: Delete file AND DB record
		os.Remove(filePath)
		DB.Exec("DELETE FROM submissions WHERE id = ?", submissionID)
		
		http.Error(w, "System error during finalization", http.StatusInternalServerError)
		return
	}

	// 7. Push to Buffered Channel
	select {
	case SubmissionQueue <- int(submissionID):
		// Success
	default:
		// Should theoretically not happen due to capacity=5000
		log.Printf("CRITICAL: Queue full! Submission %d dropped.", submissionID)
		http.Error(w, "System overloaded", http.StatusServiceUnavailable)
		return
	}

	// 8. Redirect to Status
	http.Redirect(w, r, "/status", http.StatusSeeOther)
}
