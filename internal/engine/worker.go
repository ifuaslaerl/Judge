package engine

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"github.com/ifuaslaerl/Judge/internal/data"
)

func StartWorker() {
	log.Println("WORKER: Started. Waiting for submissions...")
	for submissionID := range SubmissionQueue {
		log.Printf("WORKER: Processing Submission %d", submissionID)
		processSubmission(submissionID)
	}
}

func processSubmission(id int) {
	// 1. Fetch File Path AND Info
	var srcPath string
	var timeLimitMs, problemID int
	
	query := `
		SELECT s.file_path, p.time_limit, p.id 
		FROM submissions s
		JOIN problems p ON s.problem_id = p.id
		WHERE s.id = ?
	`
	err := data.DB.QueryRow(query, id).Scan(&srcPath, &timeLimitMs, &problemID)
	if err != nil {
		log.Printf("WORKER ERROR [Sub %d]: Could not fetch data: %v", id, err)
		return
	}

	// 2. Compilation / Prep
	ext := filepath.Ext(srcPath)
	isPython := (ext == ".py")
	var binPath string 
	
	if isPython {
		binPath = srcPath
		timeLimitMs = timeLimitMs * 2 // 2x Multiplier for Python
	} else {
		binPath = strings.Replace(srcPath, ".cpp", ".exe", 1)
		cmd := exec.Command("g++", "-O2", "-std=c++17", srcPath, "-o", binPath)
		if _, err := cmd.CombinedOutput(); err != nil {
			data.DB.Exec("UPDATE submissions SET status = 'CE' WHERE id = ?", id)
			return
		}
		defer os.Remove(binPath)
	}

	// 3. Identify Tests
	// Look for storage/problems/[id]/tests/*.in
	testDir := filepath.Join("storage", "problems", strconv.Itoa(problemID), "tests")
	tests, _ := filepath.Glob(filepath.Join(testDir, "*.in"))
	
	// Sort tests numerically if possible, or string sort otherwise
	sort.Strings(tests) 

	finalVerdict := "AC" // Optimistic default

	// 4. Execution Loop
	if len(tests) == 0 {
		// BLIND MODE: No tests defined. Run once.
		// If it doesn't crash, we give AC.
		log.Printf("WORKER [Sub %d]: No tests found. Running Blind Mode.", id)
		verdict, _ := runSecurely(id, binPath, isPython, timeLimitMs, "", "")
		if verdict != "AC" {
			finalVerdict = verdict // RTE or TLE
		}
	} else {
		// TEST MODE: Iterate over files
		for i, inPath := range tests {
			// Derive expected output path: 1.in -> 1.out
			outPath := strings.TrimSuffix(inPath, ".in") + ".out"
			
			// Temp file for user output
			userOutPath := fmt.Sprintf("/tmp/sub_%d_test_%d.out", id, i+1)

			verdict, err := runSecurely(id, binPath, isPython, timeLimitMs, inPath, userOutPath)
			
			// 1. Runtime/Time Check
			if verdict != "AC" {
				finalVerdict = fmt.Sprintf("%s on test %d", verdict, i+1)
				break
			} else if err != nil {
				// System error
				finalVerdict = "IE"
				break
			}

			// 2. Correctness Check (Comparator)
			match, err := CompareFiles(userOutPath, outPath)
			os.Remove(userOutPath) // Cleanup

			if err != nil {
				log.Printf("Comparator Error: %v", err) // Missing .out file?
				finalVerdict = "IE"
				break
			}
			
			if !match {
				finalVerdict = fmt.Sprintf("WA on test %d", i+1)
				break
			}
		}
	}

	// 5. Update DB
	log.Printf("WORKER [Sub %d]: Final Verdict -> %s", id, finalVerdict)
	data.DB.Exec("UPDATE submissions SET status = ? WHERE id = ?", finalVerdict, id)
}

// runSecurely executes the binary.
// If inputPath is empty, it runs without input redirection.
// If outputPath is provided, it redirects user stdout there.
func runSecurely(submissionID int, hostBinPath string, isPython bool, timeLimitMs int, inputPath, outputPath string) (string, error) {
	boxID := submissionID % 100
	metaFile := fmt.Sprintf("/tmp/isolate_meta_%d.txt", boxID)

	// A. Init
	exec.Command("isolate", "--cleanup", fmt.Sprintf("--box-id=%d", boxID)).Run()
	initCmd := exec.Command("isolate", "--init", fmt.Sprintf("--box-id=%d", boxID))
	boxPathBytes, _ := initCmd.Output()
	boxPath := strings.TrimSpace(string(boxPathBytes))

	// B. Copy Binary
	sandboxFilename := "program"
	runCommand := "./program"
	if isPython {
		sandboxFilename = "program.py"
		runCommand = "/usr/bin/python3 program.py"
	}
	copyFile(hostBinPath, filepath.Join(boxPath, "box", sandboxFilename))

	// C. Prepare Input (Copy .in file to sandbox if exists)
	// We bind the input file or copy it? Isolate handles stdin redirection via flags.
	// Simpler: Copy input file to /box/stdin.txt and redirect inside
	
	isolateArgs := []string{
		fmt.Sprintf("--box-id=%d", boxID),
		fmt.Sprintf("--meta=%s", metaFile),
		fmt.Sprintf("--time=%.2f", float64(timeLimitMs)/1000.0),
		"--mem=256000",
		"--processes=10",
		"--run",
	}

	if isPython {
		isolateArgs = append(isolateArgs, "--dir=/usr/", "--dir=/lib/", "--dir=/lib64/", "--dir=/etc/", "--env=HOME=/tmp")
	}

	// Handling IO redirection
	// isolate --stdin=[file inside box] --stdout=[file inside box]
	// We need to move the input file INTO the box first.
	if inputPath != "" {
		boxIn := filepath.Join(boxPath, "box", "std.in")
		copyFile(inputPath, boxIn)
		isolateArgs = append(isolateArgs, "--stdin=std.in")
	}
	
	// We want stdout captured to a file we can read later
	if outputPath != "" {
		isolateArgs = append(isolateArgs, "--stdout=std.out")
	}

	// Command
	isolateArgs = append(isolateArgs, "--", "/bin/sh", "-c", runCommand)

	// Run
	exec.Command("isolate", isolateArgs...).Run()

	// Extract Output
	if outputPath != "" {
		boxOut := filepath.Join(boxPath, "box", "std.out")
		// Copy back to host
		copyFile(boxOut, outputPath)
	}

	return parseMetaFile(metaFile)
}

func parseMetaFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil { return "IE", err }
	content := string(data)

	if strings.Contains(content, "status:TO") { return "TLE", nil }
	if strings.Contains(content, "status:RE") || strings.Contains(content, "status:SG") { return "RTE", nil }
	if strings.Contains(content, "status:XX") { return "IE", nil }
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
	return os.Chmod(dst, 0755)
}
