package tasks

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
)

// BakeTests generates .in and .out files for a problem
// Usage: --bake [problemID] [seed] [count]
func BakeTests(args []string) {
	if len(args) < 3 {
		log.Fatal("Usage: --bake [problemID] [seed] [count]")
	}

	idStr := args[0]
	seedBase, _ := strconv.Atoi(args[1])
	count, _ := strconv.Atoi(args[2])

	baseDir := filepath.Join("storage", "problems", idStr)
	testDir := filepath.Join(baseDir, "tests")
	
	genPath := filepath.Join(baseDir, "generator.py")
	solPath := filepath.Join(baseDir, "solution.cpp")
	binPath := filepath.Join(baseDir, "solution_exec")

	// 1. Validation
	if _, err := os.Stat(genPath); os.IsNotExist(err) {
		log.Fatalf("Missing generator.py in %s", baseDir)
	}
	if _, err := os.Stat(solPath); os.IsNotExist(err) {
		log.Fatalf("Missing solution.cpp in %s", baseDir)
	}

	// 2. Compile Reference Solution
	log.Println("Compiling reference solution...")
	cmd := exec.Command("g++", "-O2", solPath, "-o", binPath)
	if out, err := cmd.CombinedOutput(); err != nil {
		log.Fatalf("Compilation failed:\n%s", out)
	}
	defer os.Remove(binPath)

	// 3. Create Tests Directory
	os.MkdirAll(testDir, 0755)

	// 4. Generate Loop
	log.Printf("Baking %d tests (Seed base: %d)...", count, seedBase)
	
	for i := 1; i <= count; i++ {
		seed := seedBase + i
		inFile := filepath.Join(testDir, fmt.Sprintf("%d.in", i))
		outFile := filepath.Join(testDir, fmt.Sprintf("%d.out", i))

		// A. Run Generator -> .in
		// cmd: python3 generator.py [seed] > [i].in
		genCmd := exec.Command("python3", genPath, strconv.Itoa(seed))
		fIn, _ := os.Create(inFile)
		genCmd.Stdout = fIn
		if err := genCmd.Run(); err != nil {
			fIn.Close()
			log.Fatalf("Generator failed on test %d: %v", i, err)
		}
		fIn.Close()

		// B. Run Solution -> .out
		// cmd: ./solution < [i].in > [i].out
		solCmd := exec.Command(binPath)
		fInRead, _ := os.Open(inFile)
		fOut, _ := os.Create(outFile)
		
		solCmd.Stdin = fInRead
		solCmd.Stdout = fOut
		
		if err := solCmd.Run(); err != nil {
			log.Fatalf("Reference solution crashed on test %d: %v", i, err)
		}
		
		fInRead.Close()
		fOut.Close()
		
		fmt.Printf("\rGenerated Test %d/%d", i, count)
	}
	fmt.Println("\nDONE. Tests saved to", testDir)
}
