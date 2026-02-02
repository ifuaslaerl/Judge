package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	// 1. Initialize Database & Queue
	InitDB()
	InitQueue()
	defer DB.Close()

	// 2. Parse CLI Flags
	flushCmd := flag.Bool("flush-sessions", false, "Truncate the Sessions table and exit")
	wipeCmd := flag.Bool("wipe-all", false, "DANGER: Delete all submissions, users, and sessions") // <--- NEW	
	flag.Parse()

	// 3. Execute CLI Command if requested
	if *flushCmd {
		log.Println("EXECUTING: Flushing all active sessions...")
		FlushSessions()
		os.Exit(0)
	}

	if *wipeCmd {
		log.Println("EXECUTING: Weekly Wipe (Factory Reset)...")
		WipeAll()
		os.Exit(0)
	}

	// --- NEW: Phase 6 (The Reaper) ---
	StartReaper()

	// --- NEW: Phase 5 (The Worker) ---
	// Run in a separate goroutine so it doesn't block the server
	go StartWorker()

	// 4. Server Setup
	// Public
	http.HandleFunc("/login", HandleLogin)

	// Protected (Apply Middleware)
	http.HandleFunc("/dashboard", AuthMiddleware(HandleDashboard))
	http.HandleFunc("/status", AuthMiddleware(HandleStatus))
	http.HandleFunc("/submit/", AuthMiddleware(HandleSubmission)) //

	// Root Redirect
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
	    http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})
	
	// REMOVED DUPLICATE REGISTRATION HERE

	// Port 8443
	port := ":8443"
	certFile := "certs/server.crt"
	keyFile := "certs/server.key"

	log.Printf("Starting secure server on https://localhost%s", port)
	
	err := http.ListenAndServeTLS(port, certFile, keyFile, nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
