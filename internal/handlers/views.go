package handlers

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"github.com/ifuaslaerl/Judge/internal/auth"
        "github.com/ifuaslaerl/Judge/internal/data"
	"github.com/ifuaslaerl/Judge/internal/middleware"
)

// --- Templates ---
// In a real app, these would be in the /templates folder.
// For simplicity in this file structure, we parse them globally or per request.

func renderTemplate(w http.ResponseWriter, tmplName string, data interface{}) {
	tmplPath := filepath.Join("templates", tmplName)
	t, err := template.ParseFiles(tmplPath)
	if err != nil {
		log.Printf("Template Error: %v", err)
		http.Error(w, "Template Error", http.StatusInternalServerError)
		return
	}
	t.Execute(w, data)
}

// --- Handlers ---

// GET /login
func HandleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		processLogin(w, r)
		return
	}
	renderTemplate(w, "login.html", nil)
}

// POST /login logic
func processLogin(w http.ResponseWriter, r *http.Request) {
	username := r.FormValue("username")
	password := r.FormValue("password")

	var id int
	var hash string

	// 1. Check User
	err := data.DB.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Database Error", http.StatusInternalServerError)
		return
	}

	// 2. Compare Hash
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		return
	}

	// 3. Create Session
	token, err := auth.CreateSession(id)
	if err != nil {
		http.Error(w, "Session Creation Failed", http.StatusInternalServerError)
		return
	}

	// 4. Set Cookie (CORRECTED)
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true,      // Required for your HTTPS setup
		MaxAge:   315360000, // 10 years in seconds (Indefinite persistence)
	})

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// GET /dashboard
func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	// Fetch Problems
	rows, err := data.DB.Query("SELECT id, letter_code, time_limit, pdf_path FROM problems ORDER BY letter_code")
	if err != nil {
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Problem struct {
		ID        int
		Letter    string
		TimeLimit int
		PDF       string
	}

	var problems []Problem
	for rows.Next() {
		var p Problem
		if err := rows.Scan(&p.ID, &p.Letter, &p.TimeLimit, &p.PDF); err != nil {
			continue
		}
		problems = append(problems, p)
	}

	renderTemplate(w, "dashboard.html", problems)
}

// GET /status
func HandleStatus(w http.ResponseWriter, r *http.Request) {
	userID := r.Context().Value(middleware.UserIDKey).(int)

	// Fetch recent submissions for this user
	rows, err := data.DB.Query(`
		SELECT s.id, p.letter_code, s.status, s.created_at 
		FROM submissions s 
		JOIN problems p ON s.problem_id = p.id 
		WHERE s.user_id = ? 
		ORDER BY s.id DESC LIMIT 20`, userID)
	
	if err != nil {
		http.Error(w, "DB Error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	type Submission struct {
		ID        int
		Problem   string
		Status    string
		Time      string
	}

	var subs []Submission
	for rows.Next() {
		var s Submission
		var t time.Time
		if err := rows.Scan(&s.ID, &s.Problem, &s.Status, &t); err != nil {
			continue
		}
		s.Time = t.Format("15:04:05")
		subs = append(subs, s)
	}

	renderTemplate(w, "status.html", subs)
}

// GET /problems/[id]/pdf
func HandlePDF(w http.ResponseWriter, r *http.Request) {
	// 1. Parse ID from URL
	// Path format: /problems/[id]/pdf
	parts := strings.Split(r.URL.Path, "/")
	if len(parts) < 3 {
		http.Error(w, "Invalid URL", http.StatusBadRequest)
		return
	}

	// The ID should be the second to last part (index 2 in /problems/1/pdf)
	// We might need to adjust based on strict parsing, but let's assume standard routing
	idStr := parts[2]

	// 2. Query DB for file path
	var pdfPath string
	err := data.DB.QueryRow("SELECT pdf_path FROM problems WHERE id = ?", idStr).Scan(&pdfPath)
	if err != nil {
		http.Error(w, "Problem or PDF not found", http.StatusNotFound)
		return
	}

	// 3. Serve the file
	// Verify file exists on disk first
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		http.Error(w, "PDF file missing from storage", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/pdf")
	http.ServeFile(w, r, pdfPath)
}
