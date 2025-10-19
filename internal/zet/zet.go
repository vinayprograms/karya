package zet

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Zettel struct {
	ID    string
	Title string
	Path  string
}

type SearchResult struct {
	ZettelID string
	Title    string
	LineNum  int
	Line     string
	Path     string
}

func CreateZettel(zetDir, zetID, title string) error {
	zetPath := filepath.Join(zetDir, zetID)
	if err := os.MkdirAll(zetPath, 0755); err != nil {
		return err
	}

	readmePath := filepath.Join(zetPath, "README.md")
	content := fmt.Sprintf("# %s\n\n\n", title)
	return os.WriteFile(readmePath, []byte(content), 0644)
}

func ListZettels(zetDir string) ([]Zettel, error) {
	entries, err := os.ReadDir(zetDir)
	if err != nil {
		return nil, err
	}

	var validDirs []string
	for _, entry := range entries {
		if entry.IsDir() && IsValidZettelID(entry.Name()) {
			validDirs = append(validDirs, entry.Name())
		}
	}

	if len(validDirs) == 0 {
		return []Zettel{}, nil
	}

	numWorkers := len(validDirs)
	maxWorkers := 8
	if numWorkers > maxWorkers {
		numWorkers = maxWorkers
	}
	if numWorkers < 1 {
		numWorkers = 1
	}

	jobs := make(chan string, len(validDirs))
	results := make(chan Zettel, len(validDirs))

	for w := 0; w < numWorkers; w++ {
		go func() {
			for zetID := range jobs {
				readmePath := filepath.Join(zetDir, zetID, "README.md")
				title, err := GetZettelTitle(zetDir, zetID)
				if err != nil {
					continue
				}
				results <- Zettel{
					ID:    zetID,
					Title: title,
					Path:  readmePath,
				}
			}
		}()
	}

	for _, zetID := range validDirs {
		jobs <- zetID
	}
	close(jobs)

	var zettels []Zettel
	for i := 0; i < len(validDirs); i++ {
		select {
		case z := <-results:
			zettels = append(zettels, z)
		case <-time.After(5 * time.Second):
			break
		}
	}

	sort.Slice(zettels, func(i, j int) bool {
		return zettels[i].ID > zettels[j].ID
	})

	return zettels, nil
}

func CountZettels(zetDir string) (int, error) {
	entries, err := os.ReadDir(zetDir)
	if err != nil {
		return 0, err
	}

	count := 0
	for _, entry := range entries {
		if entry.IsDir() && IsValidZettelID(entry.Name()) {
			count++
		}
	}

	return count, nil
}

func GetZettelTitle(zetDir, zetID string) (string, error) {
	readmePath := filepath.Join(zetDir, zetID, "README.md")
	file, err := os.Open(readmePath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	if scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "# ") {
			return strings.TrimSpace(line[2:]), nil
		}
	}

	return "", fmt.Errorf("no title found")
}

func SearchZettels(zetDir, pattern string) ([]SearchResult, error) {
	zettels, err := ListZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	patternLower := strings.ToLower(pattern)

	for _, z := range zettels {
		file, err := os.Open(z.Path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if strings.Contains(strings.ToLower(line), patternLower) {
				results = append(results, SearchResult{
					ZettelID: z.ID,
					Title:    z.Title,
					LineNum:  lineNum,
					Line:     line,
					Path:     z.Path,
				})
			}
		}
		file.Close()
	}

	return results, nil
}

func SearchZettelTitles(zetDir, pattern string) ([]Zettel, error) {
	zettels, err := ListZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var results []Zettel
	patternLower := strings.ToLower(pattern)

	for _, z := range zettels {
		if strings.Contains(strings.ToLower(z.Title), patternLower) {
			results = append(results, z)
		}
	}

	return results, nil
}

func FindTodos(zetDir string) ([]SearchResult, error) {
	zettels, err := ListZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var results []SearchResult
	// Match task keywords (TODO:, TASK:, etc.)
	taskPattern := regexp.MustCompile(`^\s*(TODO|TASK|NOTE|REMINDER|EVENT|MEETING|CALL|EMAIL|MESSAGE|FOLLOWUP|REVIEW|CHECKIN|CHECKOUT|RESEARCH|READING|WRITING|DRAFT|EDITING|FINALIZE|SUBMIT|PRESENTATION|WAITING|DEFERRED|DELEGATED|DOING|INPROGRESS|STARTED|WORKING|WIP):`)

	for _, z := range zettels {
		file, err := os.Open(z.Path)
		if err != nil {
			continue
		}

		scanner := bufio.NewScanner(file)
		lineNum := 0
		for scanner.Scan() {
			lineNum++
			line := scanner.Text()
			if taskPattern.MatchString(line) {
				results = append(results, SearchResult{
					ZettelID: z.ID,
					Title:    z.Title,
					LineNum:  lineNum,
					Line:     line,
					Path:     z.Path,
				})
			}
		}
		file.Close()
	}

	return results, nil
}

func UpdateReadme(zetDir string) error {
	zettels, err := ListZettels(zetDir)
	if err != nil {
		return err
	}

	var content strings.Builder
	content.WriteString("# Index\n")

	for _, z := range zettels {
		content.WriteString(fmt.Sprintf("* [%s](./%s/README.md) - %s\n", z.ID, z.ID, z.Title))
	}

	readmePath := filepath.Join(zetDir, "README.md")
	return os.WriteFile(readmePath, []byte(content.String()), 0644)
}

func DeleteZettel(zetDir, zetID string) error {
	zetPath := filepath.Join(zetDir, zetID)

	if err := os.RemoveAll(zetPath); err != nil {
		return err
	}

	return UpdateReadme(zetDir)
}

func IsValidZettelID(id string) bool {
	if len(id) != 14 {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func FindMatchingZettels(zetDir, prefix string) ([]Zettel, error) {
	zettels, err := ListZettels(zetDir)
	if err != nil {
		return nil, err
	}

	var matches []Zettel
	for _, z := range zettels {
		if strings.HasPrefix(z.ID, prefix) {
			matches = append(matches, z)
		}
	}

	return matches, nil
}

func SearchInFile(filePath, searchTerm string) []SearchResult {
	var results []SearchResult

	file, err := os.Open(filePath)
	if err != nil {
		return results
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if strings.Contains(strings.ToLower(line), strings.ToLower(searchTerm)) {
			dir := filepath.Dir(filePath)
			zetID := filepath.Base(dir)

			results = append(results, SearchResult{
				ZettelID: zetID,
				LineNum:  lineNum,
				Line:     line,
				Path:     filePath,
			})
		}
	}
	return results
}

func GenerateZettelID() string {
	return time.Now().UTC().Format("20060102150405")
}
