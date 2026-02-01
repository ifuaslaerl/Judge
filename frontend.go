package main

import (
	"database/sql"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"time"

	"golang.org/x/crypto/bcrypt"
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
	err := DB.QueryRow("SELECT id, password_hash FROM users WHERE username = ?", username).Scan(&id, &hash)
	if err == sql.ErrNoRows {
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		return
	} else if err != nil {
		http.Error(w, "Database Error", http.StatusInternalServerError)
		return
	}

	// 2. Compare Hash (Requires: go get golang.org/x/crypto/bcrypt)
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		http.Error(w, "Invalid Credentials", http.StatusUnauthorized)
		return
	}

	// 3. Create Session
	token, err := CreateSession(id)
	if err != nil {
		http.Error(w, "Session Creation Failed", http.StatusInternalServerError)
		return
	}

	// 4. Set Cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "session_token",
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   true, // Since we run on TLS
	})

	http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
}

// GET /dashboard
func HandleDashboard(w http.ResponseWriter, r *http.Request) {
	// Fetch Problems
	rows, err := DB.Query("SELECT id, letter_code, time_limit, pdf_path FROM problems ORDER BY letter_code")
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
	userID := r.Context().Value(UserIDKey).(int)

	// Fetch recent submissions for this user
	rows, err := DB.Query(`
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
