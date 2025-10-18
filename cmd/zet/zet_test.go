package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewZettel(t *testing.T) {
	tmpDir := t.TempDir()
	
	zetID := time.Now().UTC().Format("20060102150405")
	title := "Test Zettel"
	
	err := createZettel(tmpDir, zetID, title)
	if err != nil {
		t.Fatalf("createZettel failed: %v", err)
	}
	
	zetPath := filepath.Join(tmpDir, zetID, "README.md")
	if _, err := os.Stat(zetPath); os.IsNotExist(err) {
		t.Errorf("Zettel file not created at %s", zetPath)
	}
	
	content, err := os.ReadFile(zetPath)
	if err != nil {
		t.Fatalf("Failed to read zettel: %v", err)
	}
	
	expectedContent := "# " + title + "\n\n\n"
	if string(content) != expectedContent {
		t.Errorf("Zettel content = %q, want %q", string(content), expectedContent)
	}
}

func TestListZettels(t *testing.T) {
	tmpDir := t.TempDir()
	
	zettels := []struct {
		id    string
		title string
	}{
		{"20231001120000", "First Zettel"},
		{"20231002130000", "Second Zettel"},
		{"20231003140000", "Third Zettel"},
	}
	
	for _, z := range zettels {
		err := createZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}
	
	list, err := listZettels(tmpDir)
	if err != nil {
		t.Fatalf("listZettels failed: %v", err)
	}
	
	if len(list) != len(zettels) {
		t.Errorf("listZettels returned %d zettels, want %d", len(list), len(zettels))
	}
	
	for i, z := range list {
		expectedIdx := len(zettels) - 1 - i
		if z.ID != zettels[expectedIdx].id {
			t.Errorf("Zettel %d ID = %s, want %s", i, z.ID, zettels[expectedIdx].id)
		}
		if z.Title != zettels[expectedIdx].title {
			t.Errorf("Zettel %d Title = %s, want %s", i, z.Title, zettels[expectedIdx].title)
		}
	}
}

func TestCountZettels(t *testing.T) {
	tmpDir := t.TempDir()
	
	count, err := countZettels(tmpDir)
	if err != nil {
		t.Fatalf("countZettels failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Empty dir count = %d, want 0", count)
	}
	
	for i := 0; i < 5; i++ {
		zetID := time.Now().UTC().Add(time.Duration(i) * time.Second).Format("20060102150405")
		err := createZettel(tmpDir, zetID, "Test")
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}
	
	count, err = countZettels(tmpDir)
	if err != nil {
		t.Fatalf("countZettels failed: %v", err)
	}
	if count != 5 {
		t.Errorf("count = %d, want 5", count)
	}
}

func TestSearchZettels(t *testing.T) {
	tmpDir := t.TempDir()
	
	zettels := []struct {
		id      string
		title   string
		content string
	}{
		{"20231001120000", "First Zettel", "# First Zettel\n\nThis is about golang programming.\n"},
		{"20231002130000", "Second Zettel", "# Second Zettel\n\nThis discusses rust language.\n"},
		{"20231003140000", "Third Zettel", "# Third Zettel\n\nMore about golang and testing.\n"},
	}
	
	for _, z := range zettels {
		zetPath := filepath.Join(tmpDir, z.id)
		os.MkdirAll(zetPath, 0755)
		os.WriteFile(filepath.Join(zetPath, "README.md"), []byte(z.content), 0644)
	}
	
	results, err := searchZettels(tmpDir, "golang")
	if err != nil {
		t.Fatalf("searchZettels failed: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("searchZettels returned %d results, want 2", len(results))
	}
}

func TestSearchZettelTitles(t *testing.T) {
	tmpDir := t.TempDir()
	
	zettels := []struct {
		id    string
		title string
	}{
		{"20231001120000", "Golang Programming"},
		{"20231002130000", "Rust Language"},
		{"20231003140000", "Golang Testing"},
	}
	
	for _, z := range zettels {
		err := createZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}
	
	results, err := searchZettelTitles(tmpDir, "Golang")
	if err != nil {
		t.Fatalf("searchZettelTitles failed: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("searchZettelTitles returned %d results, want 2", len(results))
	}
}

func TestGetZettelTitle(t *testing.T) {
	tmpDir := t.TempDir()
	zetID := "20231001120000"
	title := "Test Zettel Title"
	
	err := createZettel(tmpDir, zetID, title)
	if err != nil {
		t.Fatalf("createZettel failed: %v", err)
	}
	
	gotTitle, err := getZettelTitle(tmpDir, zetID)
	if err != nil {
		t.Fatalf("getZettelTitle failed: %v", err)
	}
	
	if gotTitle != title {
		t.Errorf("getZettelTitle = %q, want %q", gotTitle, title)
	}
}

func TestFindTodos(t *testing.T) {
	tmpDir := t.TempDir()
	
	content := `# Test Zettel

Some content here.

- [ ] First todo item
- [x] Completed item
- [ ] Second todo item

More content.
`
	
	zetID := "20231001120000"
	zetPath := filepath.Join(tmpDir, zetID)
	os.MkdirAll(zetPath, 0755)
	os.WriteFile(filepath.Join(zetPath, "README.md"), []byte(content), 0644)
	
	results, err := findTodos(tmpDir)
	if err != nil {
		t.Fatalf("findTodos failed: %v", err)
	}
	
	if len(results) != 2 {
		t.Errorf("findTodos returned %d results, want 2", len(results))
	}
}

func TestUpdateReadme(t *testing.T) {
	tmpDir := t.TempDir()
	
	zettels := []struct {
		id    string
		title string
	}{
		{"20231001120000", "First Zettel"},
		{"20231002130000", "Second Zettel"},
	}
	
	for _, z := range zettels {
		err := createZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}
	
	err := updateReadme(tmpDir)
	if err != nil {
		t.Fatalf("updateReadme failed: %v", err)
	}
	
	readmePath := filepath.Join(tmpDir, "README.md")
	if _, err := os.Stat(readmePath); os.IsNotExist(err) {
		t.Errorf("README.md not created")
	}
	
	content, err := os.ReadFile(readmePath)
	if err != nil {
		t.Fatalf("Failed to read README.md: %v", err)
	}
	
	contentStr := string(content)
	if !contains(contentStr, "# Index") {
		t.Errorf("README.md missing '# Index' heading")
	}
	if !contains(contentStr, "20231001120000") {
		t.Errorf("README.md missing first zettel ID")
	}
	if !contains(contentStr, "First Zettel") {
		t.Errorf("README.md missing first zettel title")
	}
}

func TestValidateZettelID(t *testing.T) {
	tests := []struct {
		id    string
		valid bool
	}{
		{"20231001120000", true},
		{"20231231235959", true},
		{"2023100112000", false},
		{"202310011200000", false},
		{"abcd1001120000", false},
		{"", false},
	}
	
	for _, tt := range tests {
		got := isValidZettelID(tt.id)
		if got != tt.valid {
			t.Errorf("isValidZettelID(%q) = %v, want %v", tt.id, got, tt.valid)
		}
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestFindMatchingZettels(t *testing.T) {
	tmpDir := t.TempDir()
	
	zettels := []struct {
		id    string
		title string
	}{
		{"20231001120000", "First Zettel"},
		{"20231001130000", "Second Zettel"},
		{"20231002140000", "Third Zettel"},
	}
	
	for _, z := range zettels {
		err := createZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}
	
	matches, err := findMatchingZettels(tmpDir, "20231001")
	if err != nil {
		t.Fatalf("findMatchingZettels failed: %v", err)
	}
	
	if len(matches) != 2 {
		t.Errorf("findMatchingZettels returned %d matches, want 2", len(matches))
	}
	
	matches, err = findMatchingZettels(tmpDir, "202310011200")
	if err != nil {
		t.Fatalf("findMatchingZettels failed: %v", err)
	}
	
	if len(matches) != 1 {
		t.Errorf("findMatchingZettels returned %d matches, want 1", len(matches))
	}
	
	matches, err = findMatchingZettels(tmpDir, "20239999")
	if err != nil {
		t.Fatalf("findMatchingZettels failed: %v", err)
	}
	
	if len(matches) != 0 {
		t.Errorf("findMatchingZettels returned %d matches, want 0", len(matches))
	}
}

func TestGitCommit(t *testing.T) {
	tmpDir := t.TempDir()
	
	err := gitCommit(tmpDir, "20231001120000", "Test")
	if err != nil {
		t.Errorf("gitCommit should not fail when .git doesn't exist: %v", err)
	}
}

func TestShowZettel(t *testing.T) {
	tmpDir := t.TempDir()
	zetID := "20231001120000"
	title := "Test Zettel"
	
	err := createZettel(tmpDir, zetID, title)
	if err != nil {
		t.Fatalf("createZettel failed: %v", err)
	}
	
	err = showZettel(tmpDir, zetID)
	if err != nil {
		t.Errorf("showZettel failed: %v", err)
	}
	
	err = showZettel(tmpDir, "99999999999999")
	if err == nil {
		t.Errorf("showZettel should fail for non-existent zettel")
	}
}

func TestPrintSearchResults(t *testing.T) {
	results := []SearchResult{
		{ZettelID: "20231001120000", Title: "Test", LineNum: 1, Line: "test line"},
	}
	
	printSearchResults(results)
	
	printSearchResults([]SearchResult{})
}

func TestPrintTitleSearchResults(t *testing.T) {
	results := []Zettel{
		{ID: "20231001120000", Title: "Test Zettel"},
	}
	
	printTitleSearchResults(results)
}

func TestSubstringSearch(t *testing.T) {
	tmpDir := t.TempDir()
	
	zettels := []struct {
		id      string
		title   string
		content string
	}{
		{"20231001120000", "Golang Programming", "# Golang Programming\n\nThis is about golang programming.\n"},
		{"20231002130000", "Rust Language", "# Rust Language\n\nThis discusses rust language.\n"},
		{"20231003140000", "Go Testing", "# Go Testing\n\nMore about golang and testing.\n"},
	}
	
	for _, z := range zettels {
		zetPath := filepath.Join(tmpDir, z.id)
		os.MkdirAll(zetPath, 0755)
		os.WriteFile(filepath.Join(zetPath, "README.md"), []byte(z.content), 0644)
	}
	
	// Test case-insensitive substring search
	results, err := searchZettels(tmpDir, "go")
	if err != nil {
		t.Fatalf("searchZettels failed: %v", err)
	}
	
	// Should match "golang" in content and titles, and "Go" in title
	// Each zettel with "go" substring will have multiple matches (title line + content)
	if len(results) < 2 {
		t.Errorf("searchZettels('go') returned %d results, want at least 2", len(results))
	}
	
	// Verify it's case-insensitive
	results, err = searchZettels(tmpDir, "GOLANG")
	if err != nil {
		t.Fatalf("searchZettels failed: %v", err)
	}
	
	if len(results) < 2 {
		t.Errorf("searchZettels('GOLANG') returned %d results, want at least 2 (case-insensitive)", len(results))
	}
	
	// Test title search
	titleResults, err := searchZettelTitles(tmpDir, "go")
	if err != nil {
		t.Fatalf("searchZettelTitles failed: %v", err)
	}
	
	if len(titleResults) != 2 {
		t.Errorf("searchZettelTitles('go') returned %d results, want 2 (should match 'Golang' and 'Go')", len(titleResults))
	}
	
	// Test case-insensitive title search
	titleResults, err = searchZettelTitles(tmpDir, "TEST")
	if err != nil {
		t.Fatalf("searchZettelTitles failed: %v", err)
	}
	
	if len(titleResults) != 1 {
		t.Errorf("searchZettelTitles('TEST') returned %d results, want 1 (case-insensitive)", len(titleResults))
	}
	
	// Test partial word matching
	titleResults, err = searchZettelTitles(tmpDir, "lang")
	if err != nil {
		t.Fatalf("searchZettelTitles failed: %v", err)
	}
	
	if len(titleResults) != 2 {
		t.Errorf("searchZettelTitles('lang') returned %d results, want 2 (should match 'Golang' and 'Language')", len(titleResults))
	}
}
