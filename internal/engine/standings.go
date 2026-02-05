package engine

import (
	"sort"
	"sync"
	"time"
	"github.com/ifuaslaerl/Judge/internal/data"
)

// --- Structs ---

type ProblemMeta struct {
	ID     int
	Letter string
}

type Cell struct {
	Solved    bool
	Attempts  int    // Number of WAs before AC (or total tries if not solved)
	Time      string // Formatted time of AC (or blank)
	IsPending bool   // Visual cue
}

type RankRow struct {
	Rank        int
	DisplayName string
	Solved      int
	Penalty     int // In minutes
	Cells       map[string]Cell
}

type Scoreboard struct {
	Problems []ProblemMeta
	Rows     []RankRow
	LastUpd  time.Time
}

// --- Caching ---

var (
	cache      *Scoreboard
	cacheMutex sync.RWMutex
	cacheTTL   = 30 * time.Second
)

// --- Logic ---

func GetScoreboard() (*Scoreboard, error) {
	cacheMutex.RLock()
	if cache != nil && time.Since(cache.LastUpd) < cacheTTL {
		defer cacheMutex.RUnlock()
		return cache, nil
	}
	cacheMutex.RUnlock() // Release read lock to acquire write lock

	return generateScoreboard()
}

func generateScoreboard() (*Scoreboard, error) {
	cacheMutex.Lock()
	defer cacheMutex.Unlock()

	// Double-check cache inside lock to prevent race condition
	if cache != nil && time.Since(cache.LastUpd) < cacheTTL {
		return cache, nil
	}

	// 1. Fetch Problems
	pRows, err := data.DB.Query("SELECT id, letter_code FROM problems ORDER BY letter_code")
	if err != nil {
		return nil, err
	}
	defer pRows.Close()

	var problems []ProblemMeta
	probMap := make(map[int]string) // ID -> Letter
	for pRows.Next() {
		var p ProblemMeta
		if err := pRows.Scan(&p.ID, &p.Letter); err != nil { continue }
		problems = append(problems, p)
		probMap[p.ID] = p.Letter
	}

	// 2. Fetch Users
	uRows, err := data.DB.Query("SELECT id, display_name FROM users")
	if err != nil {
		return nil, err
	}
	defer uRows.Close()

	// Map: UserID -> Row Pointer
	userMap := make(map[int]*RankRow)
	var rows []*RankRow

	for uRows.Next() {
		var uid int
		var name string
		if err := uRows.Scan(&uid, &name); err != nil { continue }
		
		r := &RankRow{
			DisplayName: name,
			Cells:       make(map[string]Cell),
		}
		// Initialize empty cells for all problems
		for _, p := range problems {
			r.Cells[p.Letter] = Cell{Solved: false, Attempts: 0}
		}
		
		userMap[uid] = r
		rows = append(rows, r)
	}

	// 3. Fetch Submissions (Ordered by time to process logic chronologically)
	// We only care about: Who, What, Status, Time
	sRows, err := data.DB.Query(`
		SELECT user_id, problem_id, status, 
		(strftime('%s', created_at) / 60) as minutes -- simplified relative time
		FROM submissions ORDER BY id ASC
	`)
	if err != nil {
		return nil, err
	}
	defer sRows.Close()

	for sRows.Next() {
		var uid, pid, mins int
		var status string
		if err := sRows.Scan(&uid, &pid, &status, &mins); err != nil { continue }

		row, uExists := userMap[uid]
		letter, pExists := probMap[pid]

		if !uExists || !pExists { continue }

		cell := row.Cells[letter]

		// If already solved, ignore future submissions
		if cell.Solved {
			continue
		}

		if status == "AC" {
			cell.Solved = true
			cell.Time = formatTime(mins) // e.g. "120"
			
			// Update Row Stats
			row.Solved++
			// Penalty = Time + (20 * Previous Wrong Attempts)
			row.Penalty += mins + (20 * cell.Attempts)
		} else if status == "PENDING" {
			cell.IsPending = true
		} else if status != "CE" { // Ignore Compile Errors for penalty? Standard rules usually ignore CE.
			cell.Attempts++ // WA, TLE, RTE count as penalty attempt
		}
		
		row.Cells[letter] = cell
	}

	// 4. Sort Rows
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Solved != rows[j].Solved {
			return rows[i].Solved > rows[j].Solved // More solved = better
		}
		return rows[i].Penalty < rows[j].Penalty // Less penalty = better
	})

	// 5. Assign Ranks & Convert to Value Slice
	finalRows := make([]RankRow, len(rows))
	for i, r := range rows {
		r.Rank = i + 1
		finalRows[i] = *r
	}

	newBoard := &Scoreboard{
		Problems: problems,
		Rows:     finalRows,
		LastUpd:  time.Now(),
	}

	cache = newBoard
	return newBoard, nil
}

// formatTime converts raw minutes (from epoch) to something readable if needed.
// For a real contest, you'd subtract StartTime from 'mins'. 
// For this MVP, we just show the raw minutes integer.
func formatTime(m int) string {
	// Simple integer string for MVP
	// In Phase 10 (Contest Control), this will be (m - start_time)
	// For now, it's just a raw number, but returning string allows flexibility.
    // We fetch a simplified "minutes since epoch" / 60 from SQLite, 
    // effectively a large number. Let's assume the user knows this is raw.
    // Ideally, calculate modulo contest start. 
	return "" 
}
