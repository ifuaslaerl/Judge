package engine

import (
	"bufio"
	"os"
)

// CompareFiles performs a token-based diff (ignoring whitespace).
// Returns true if files are semantically identical.
func CompareFiles(userPath, expectedPath string) (bool, error) {
	f1, err := os.Open(userPath)
	if err != nil {
		return false, err
	}
	defer f1.Close()

	f2, err := os.Open(expectedPath)
	if err != nil {
		return false, err
	}
	defer f2.Close()

	s1 := bufio.NewScanner(f1)
	s2 := bufio.NewScanner(f2)

	s1.Split(bufio.ScanWords)
	s2.Split(bufio.ScanWords)

	for {
		t1 := s1.Scan()
		t2 := s2.Scan()

		// If one ends before the other, they are different (unless both end)
		if t1 != t2 {
			return false, nil
		}

		// Both ended successfully?
		if !t1 {
			if err1, err2 := s1.Err(), s2.Err(); err1 != nil || err2 != nil {
				return false, nil // Read error treated as mismatch
			}
			return true, nil
		}

		// Compare tokens
		if s1.Text() != s2.Text() {
			return false, nil
		}
	}
}
