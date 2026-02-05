package auth

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"github.com/ifuaslaerl/Judge/internal/data"
)

// GenerateSecureToken creates a random 32-byte hex string
func GenerateSecureToken() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		log.Fatalf("Fatal: Could not generate random token: %v", err)
	}
	return hex.EncodeToString(b)
}

// CreateSession generates a token for a user and saves it to the DB
func CreateSession(userID int) (string, error) {
	token := GenerateSecureToken()

	// Insert into sessions table
	// We use OR REPLACE or simple INSERT. Since token is PK, collision is virtually impossible.
	query := `INSERT INTO sessions (token, user_id) VALUES (?, ?)`
	_, err := data.DB.Exec(query, token, userID)
	if err != nil {
		return "", err
	}
	return token, nil
}

// GetUserFromSession validates a token and returns the user ID
func GetUserFromSession(token string) (int, bool) {
	var userID int
	query := `SELECT user_id FROM sessions WHERE token = ?`
	
	err := data.DB.QueryRow(query, token).Scan(&userID)
	if err != nil {
		return 0, false // Token not found or error
	}
	return userID, true
}

// FlushSessions deletes all active session tokens (Admin CLI)
func FlushSessions() {
	query := `DELETE FROM sessions`
	_, err := data.DB.Exec(query)
	if err != nil {
		log.Fatalf("Failed to flush sessions: %v", err)
	}
	log.Println("SUCCESS: All sessions have been flushed. Users must log in again.")
}
