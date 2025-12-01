package goal

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// Goal represents a single goal with its metadata and content
type Goal struct {
	ID        string // Unique identifier for the goal  
	Title     string
	Content   string
	CreatedAt time.Time
	UpdatedAt time.Time
	Project   string // This helps organize goals under projects
}

// Horizon represents different time horizons for goals
type Horizon string

const (
	HorizonMonthly   Horizon = "monthly"
	HorizonQuarterly Horizon = "quarterly"
	HorizonYearly    Horizon = "yearly"
	HorizonShortTerm Horizon = "short-term"
	HorizonLongTerm  Horizon = "long-term"
)

// GoalManager manages all goal operations
type GoalManager struct {
	RootDir string
}

// NewGoalManager creates a new GoalManager instance
func NewGoalManager(rootDir string) *GoalManager {
	return &GoalManager{
		RootDir: rootDir,
	}
}

// sanitizeFilename replaces special characters in filename with underscores
func sanitizeFilename(filename string) string {
	// Replace all non-alphanumeric characters (except spaces, hyphens, and underscores) with underscores
	reg := regexp.MustCompile(`[^a-zA-Z0-9 \-_]`)
	sanitized := reg.ReplaceAllString(filename, "_")
	// Then replace spaces with underscores
	sanitized = strings.ReplaceAll(sanitized, " ", "_")
	return sanitized
}

// extractTitleFromFile reads the first markdown header from a file
func extractTitleFromFile(filePath string) string {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return ""
	}
	
	lines := strings.Split(string(content), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "#"))
		}
	}
	
	return ""
}

// GetHorizonPath returns the path for a specific horizon
func (gm *GoalManager) GetHorizonPath(horizon Horizon) string {
	return filepath.Join(gm.RootDir, string(horizon))
}

// GetGoalPath returns the path for a goal file within a horizons directory
func (gm *GoalManager) GetGoalPath(horizon Horizon, period string, goalID string) string {
	horizonPath := gm.GetHorizonPath(horizon)
	periodPath := filepath.Join(horizonPath, period)
	return filepath.Join(periodPath, fmt.Sprintf("%s.md", sanitizeFilename(goalID)))
}

// CreateGoal creates a new goal file with the given title
func (gm *GoalManager) CreateGoal(horizon Horizon, period string, title string) error {
	goalPath := gm.GetGoalPath(horizon, period, title)
	
	// Create directories if they don't exist
	dir := filepath.Dir(goalPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	
	// Create the goal file with just the title
	content := fmt.Sprintf("# %s\n\n", title)
	
	return os.WriteFile(goalPath, []byte(content), 0644)
}

// ListGoals returns all goals organized by horizon and period
func (gm *GoalManager) ListGoals() (map[Horizon]map[string][]string, error) {
	goals := make(map[Horizon]map[string][]string)
	
	for _, horizon := range []Horizon{
		HorizonMonthly,
		HorizonQuarterly,
		HorizonYearly,
		HorizonShortTerm,
		HorizonLongTerm,
	} {
		horizonPath := gm.GetHorizonPath(horizon)
		horizons, err := os.ReadDir(horizonPath)
		if err != nil {
			continue
		}
		
		goals[horizon] = make(map[string][]string)
		for _, horizonDir := range horizons {
			if !horizonDir.IsDir() {
				continue
			}
			
			period := horizonDir.Name()
			goals[horizon][period] = []string{}
			
			periodPath := filepath.Join(horizonPath, period)
			goalFiles, err := os.ReadDir(periodPath)
			if err != nil {
				continue
			}
			
			for _, file := range goalFiles {
				if !file.IsDir() && strings.HasSuffix(file.Name(), ".md") {
					// Read the title from the file content
					filePath := filepath.Join(periodPath, file.Name())
					goalTitle := extractTitleFromFile(filePath)
					if goalTitle != "" {
						goals[horizon][period] = append(goals[horizon][period], goalTitle)
					}
				}
			}
		}
	}
	
	return goals, nil
}

// ListGoalsByHorizon returns all goals for a specific horizon
// If a file's title has changed, the file is renamed to match
func (gm *GoalManager) ListGoalsByHorizon(horizon Horizon) (map[string][]string, error) {
	horizonPath := gm.GetHorizonPath(horizon)
	horizons, err := os.ReadDir(horizonPath)
	if err != nil {
		return nil, err
	}
	
	goals := make(map[string][]string)
	for _, horizonDir := range horizons {
		if !horizonDir.IsDir() {
			continue
		}
		
		period := horizonDir.Name()
		goals[period] = []string{}
		
		periodPath := filepath.Join(horizonPath, period)
		goalFiles, err := os.ReadDir(periodPath)
		if err != nil {
			continue
		}
		
		for _, file := range goalFiles {
			if !file.IsDir() && strings.HasSuffix(file.Name(), ".md") {
				filePath := filepath.Join(periodPath, file.Name())
				goalTitle := extractTitleFromFile(filePath)
				if goalTitle == "" {
					continue
				}
				
				// Check if filename matches the title
				expectedFilename := sanitizeFilename(goalTitle) + ".md"
				if file.Name() != expectedFilename {
					// Title changed - rename the file
					newPath := filepath.Join(periodPath, expectedFilename)
					if err := os.Rename(filePath, newPath); err == nil {
						// Rename succeeded
					}
					// Continue with the title regardless of rename success
				}
				
				goals[period] = append(goals[period], goalTitle)
			}
		}
	}
	
	return goals, nil
}

// GetGoalPathForHorizon returns the file path for a goal at a given horizon, period, and title
func (gm *GoalManager) GetGoalPathForHorizon(horizon Horizon, period string, title string) string {
	return gm.GetGoalPath(horizon, period, title)
}