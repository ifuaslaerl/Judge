package tasks

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"

	"golang.org/x/crypto/bcrypt"
	"github.com/ifuaslaerl/Judge/internal/data"
)

// AddUser generates a random user and prints credentials to stdout
func AddUser() {
	// 1. Generate Random Credentials
	// 3 bytes = 6 hex characters
	userBytes := make([]byte, 3)
	passBytes := make([]byte, 3)

	if _, err := rand.Read(userBytes); err != nil {
		log.Fatalf("Failed to generate random bytes: %v", err)
	}
	if _, err := rand.Read(passBytes); err != nil {
		log.Fatalf("Failed to generate random bytes: %v", err)
	}

	username := "user_" + hex.EncodeToString(userBytes)
	password := hex.EncodeToString(passBytes)

	// 2. Hash Password
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		log.Fatalf("Failed to hash password: %v", err)
	}

	// 3. Insert into Database
	// We set display_name = username by default
	query := `INSERT INTO users (username, password_hash, display_name) VALUES (?, ?, ?)`
	_, err = data.DB.Exec(query, username, string(hash), username)
	if err != nil {
		log.Fatalf("DB Error: Failed to create user: %v", err)
	}

	// 4. Output to Console
	fmt.Println("========================================")
	fmt.Println("       NEW USER ACCOUNT CREATED")
	fmt.Println("========================================")
	fmt.Printf(" Username : %s\n", username)
	fmt.Printf(" Password : %s\n", password)
	fmt.Println("========================================")
}
