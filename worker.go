package main

import (
	"log"
	"os/exec"
	"path/filepath"
	"strings"
)

// StartWorker consumes the SubmissionQueue and processes code
func StartWorker() {
	log.Println("WORKER: Started. Waiting for submissions...")
	
	for submissionID := range SubmissionQueue {
		log.Printf("WORKER: Processing Submission %d", submissionID)
		processSubmission(submissionID)
	}
}

func processSubmission(id int) {
	// 1. Fetch File Path from DB
	var srcPath string
	err := DB.QueryRow("SELECT file_path FROM submissions WHERE id = ?", id).Scan(&srcPath)
	if err != nil {
		log.Printf("WORKER ERROR [Sub %d]: Could not fetch file path: %v", id, err)
		return
	}

	// 2. Compilation Step (using g++)
	// Output binary to a temporary 'bin' folder or same dir
	binPath := strings.Replace(srcPath, ".cpp", ".exe", 1) // Simple rename for binary
	
	// Command: g++ -O2 -std=c++17 [source] -o [bin]
	cmd := exec.Command("g++", "-O2", "-std=c++17", srcPath, "-o", binPath)
	output, err := cmd.CombinedOutput()

	var newStatus string
	if err != nil {
		log.Printf("WORKER [Sub %d]: Compilation Failed", id)
		log.Println(string(output)) // Log compiler errors
		newStatus = "CE" // Compilation Error
	} else {
		log.Printf("WORKER [Sub %d]: Compilation Success", id)
		newStatus = "AC" // Placeholder: Accepted (Since we don't have test cases yet)
		// Clean up binary after run
		// os.Remove(binPath) 
	}

	// 3. Update Database with Verdict
	_, err = DB.Exec("UPDATE submissions SET status = ? WHERE id = ?", newStatus, id)
	if err != nil {
		log.Printf("WORKER CRITICAL [Sub %d]: Failed to update status: %v", id, err)
	}
}
