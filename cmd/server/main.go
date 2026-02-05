package main

import (
	"flag"
	"log"
	"net/http"
	"os"

	"github.com/ifuaslaerl/Judge/internal/auth"
	"github.com/ifuaslaerl/Judge/internal/data"
	"github.com/ifuaslaerl/Judge/internal/engine"
	"github.com/ifuaslaerl/Judge/internal/handlers"
	"github.com/ifuaslaerl/Judge/internal/middleware"
	"github.com/ifuaslaerl/Judge/internal/tasks"
)

func main() {
	// 1. Initialize Database & Queue
	data.InitDB()
	engine.InitQueue()
	defer data.DB.Close()

	// 2. Parse CLI Flags
	flushCmd := flag.Bool("flush-sessions", false, "Truncate the Sessions table and exit")
	wipeCmd := flag.Bool("wipe-all", false, "DANGER: Delete all submissions, users, and sessions")
	flag.Parse()

	// 3. Execute CLI Command if requested
	if *flushCmd {
		log.Println("EXECUTING: Flushing all active sessions...")
		auth.FlushSessions() // You might need to move this from auth to tasks or export it
		os.Exit(0)
	}

	if *wipeCmd {
		log.Println("EXECUTING: Weekly Wipe (Factory Reset)...")
		tasks.WipeAll()
		os.Exit(0)
	}

	// Start Background Tasks
	tasks.StartReaper()
	go engine.StartWorker()

	// 4. Server Setup
	// Public
	http.HandleFunc("/login", handlers.HandleLogin)

	// Protected
	http.HandleFunc("/dashboard", middleware.AuthMiddleware(handlers.HandleDashboard))
	http.HandleFunc("/status", middleware.AuthMiddleware(handlers.HandleStatus))
	http.HandleFunc("/submit/", middleware.AuthMiddleware(handlers.HandleSubmission))
	http.HandleFunc("/problems/", middleware.AuthMiddleware(handlers.HandlePDF))
	    
	// Phase 8 Additions:
	http.HandleFunc("/problems/all", middleware.AuthMiddleware(handlers.HandleManualBook)) 
	http.HandleFunc("/problems/view/", middleware.AuthMiddleware(handlers.HandleProblemView))

	// Root Redirect
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusSeeOther)
	})

	// Port & Certs
	// Note: Paths are relative to where you RUN the command, usually the project root.
	port := ":8443"
	certFile := "certs/server.crt"
	keyFile := "certs/server.key"

	log.Printf("Starting secure server on https://localhost%s", port)
	err := http.ListenAndServeTLS(port, certFile, keyFile, nil)
	if err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
