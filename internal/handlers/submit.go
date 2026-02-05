package handlers

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"github.com/ifuaslaerl/Judge/internal/data"
	"github.com/ifuaslaerl/Judge/internal/engine"
	"github.com/ifuaslaerl/Judge/internal/middleware"
)

// HandleSubmission processes the upload: POST /submit/[problem_id]
func HandleSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID, ok := r.Context().Value(middleware.UserIDKey).(int)
	if !ok {
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	cleanPath := strings.TrimSuffix(r.URL.Path, "/")
	parts := strings.Split(cleanPath, "/")
	
	if len(parts) == 0 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}
	problemID, err := strconv.Atoi(parts[len(parts)-1])
	if err != nil {
		http.Error(w, "Invalid Problem ID", http.StatusBadRequest)
		return
	}

	// Enforce Submission Cap
	var count int
	err = data.DB.QueryRow("SELECT COUNT(*) FROM submissions WHERE user_id = ?", userID).Scan(&count)
	if err != nil {
		log.Printf("DB Error: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	if count >= 100 {
		http.Error(w, "Submission limit reached (Max 100). Contact Admin.", http.StatusForbidden)
		return
	}

	const maxUploadSize = (1024 * 1024) + 4096 
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadSize)
	if err := r.ParseMultipartForm(maxUploadSize); err != nil {
		http.Error(w, "File too large (Max 1MB)", http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("code")
	if err != nil {
		http.Error(w, "Failed to retrieve 'code' file", http.StatusBadRequest)
		return
	}
	defer file.Close()

    // --- PHASE 8 UPDATE: Detect Extension ---
    ext := strings.ToLower(filepath.Ext(header.Filename))
    if ext != ".cpp" && ext != ".py" {
        http.Error(w, "Only .cpp and .py files are allowed", http.StatusBadRequest)
        return
    }

	res, err := data.DB.Exec(`INSERT INTO submissions (user_id, problem_id, status, file_path) VALUES (?, ?, 'PENDING', '')`, userID, problemID)
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

    // --- PHASE 8 UPDATE: Use detected extension in filename ---
	filename := fmt.Sprintf("%d%s", submissionID, ext)
	filePath := filepath.Join("storage", "submissions", filename)

	dst, err := os.Create(filePath)
	if err != nil {
		log.Printf("Disk Write Error: %v. Rolling back submission %d", err, submissionID)
		data.DB.Exec("DELETE FROM submissions WHERE id = ?", submissionID)
		http.Error(w, "Storage failure", http.StatusInternalServerError)
		return
	}

	if _, err := io.Copy(dst, file); err != nil {
		dst.Close()
		log.Printf("File Copy Error: %v. Rolling back submission %d", err, submissionID)
		os.Remove(filePath)
		data.DB.Exec("DELETE FROM submissions WHERE id = ?", submissionID)
		http.Error(w, "Storage failure", http.StatusInternalServerError)
		return
	}
	dst.Close()

	if _, err := data.DB.Exec("UPDATE submissions SET file_path = ? WHERE id = ?", filePath, submissionID); err != nil {
		log.Printf("CRITICAL: Failed to link file path. Rolling back submission %d. Error: %v", submissionID, err)
		os.Remove(filePath)
		data.DB.Exec("DELETE FROM submissions WHERE id = ?", submissionID)
		http.Error(w, "System error during finalization", http.StatusInternalServerError)
		return
	}

	select {
	case engine.SubmissionQueue <- int(submissionID):
	default:
		log.Printf("CRITICAL: Queue full! Submission %d dropped.", submissionID)
		http.Error(w, "System overloaded", http.StatusServiceUnavailable)
		return
	}

	http.Redirect(w, r, "/status", http.StatusSeeOther)
}
