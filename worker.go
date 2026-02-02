package main

import (
	"fmt"
	"io"
	"log"
	"os"
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

	// 2. Compilation (Host Side)
	binPath := strings.Replace(srcPath, ".cpp", ".exe", 1)
	cmd := exec.Command("g++", "-O2", "-std=c++17", srcPath, "-o", binPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Printf("WORKER [Sub %d]: Compilation Failed\n%s", id, out)
		DB.Exec("UPDATE submissions SET status = 'CE' WHERE id = ?", id)
		return
	}

	// 3. Execution (Sandbox)
	log.Printf("WORKER [Sub %d]: Compilation Success. Starting Sandbox...", id)
	verdict, runErr := runSecurely(id, binPath)

	if runErr != nil {
		// Log internal errors (like sandbox init failure), but don't crash
		log.Printf("WORKER CRITICAL [Sub %d]: Sandbox system error: %v", id, runErr)
	}

	// 4. Update DB
	log.Printf("WORKER [Sub %d]: Final Verdict -> %s", id, verdict)
	DB.Exec("UPDATE submissions SET status = ? WHERE id = ?", verdict, id)

	// Cleanup binary
	os.Remove(binPath)
}

func runSecurely(submissionID int, hostBinPath string) (string, error) {
	// Use (ID % 100) to recycle boxes 0-99
	boxID := submissionID % 100
	metaFile := fmt.Sprintf("/tmp/isolate_meta_%d.txt", boxID)

	// A. Cleanup & Init Box
	// Reliance on system PATH for "isolate"
	exec.Command("isolate", "--cleanup", fmt.Sprintf("--box-id=%d", boxID)).Run()
	
	initCmd := exec.Command("isolate", "--init", fmt.Sprintf("--box-id=%d", boxID))
	boxPathBytes, err := initCmd.Output()
	if err != nil {
		return "IE", fmt.Errorf("init failed (check PATH/Permissions): %v", err)
	}
	boxPath := strings.TrimSpace(string(boxPathBytes))

	// B. Copy Binary to Sandbox
	// Target: [boxPath]/box/run_program
	sandboxBinPath := filepath.Join(boxPath, "box", "program")
	if err := copyFile(hostBinPath, sandboxBinPath); err != nil {
		return "IE", fmt.Errorf("copy failed: %v", err)
	}

	// C. Execute with Metadata
	// --meta saves execution stats (time, memory, exit reason) to a file
	runCmd := exec.Command("isolate",
		fmt.Sprintf("--box-id=%d", boxID),
		fmt.Sprintf("--meta=%s", metaFile), 
		"--time=2.0",
		"--mem=256000",
		"--run",
		"--",
		"./program",
	)

	// We ignore the error from Run() because we rely on the meta file for the verdict.
	// (isolate returns exit code 1 on TLE/RTE, which is expected behavior)
	_ = runCmd.Run()

	// D. Parse Metadata to determine AC vs TLE vs RTE
	return parseMetaFile(metaFile)
}

func parseMetaFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "IE", err // Internal Error if meta file is missing
	}

	content := string(data)
	
	// Check "status" field in the meta file
	// status:TO -> Time Limit Exceeded
	// status:RE -> Runtime Error
	// status:SG -> Signal (Crashed)
	// (If "status" is missing, it means AC)

	if strings.Contains(content, "status:TO") {
		return "TLE", nil
	}
	if strings.Contains(content, "status:RE") || strings.Contains(content, "status:SG") {
		return "RTE", nil
	}
	if strings.Contains(content, "status:XX") {
		return "IE", nil // Internal configuration error
	}

	return "AC", nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()

	if _, err = io.Copy(out, in); err != nil { return err }
	
	// Ensure executable permissions
	return os.Chmod(dst, 0755)
}
