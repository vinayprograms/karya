package zet

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestCreateZettel(t *testing.T) {
	tmpDir := t.TempDir()

	zetID := time.Now().UTC().Format("20060102150405")
	title := "Test Zettel"

	err := CreateZettel(tmpDir, zetID, title)
	if err != nil {
		t.Fatalf("CreateZettel failed: %v", err)
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
		err := CreateZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}

	list, err := ListZettels(tmpDir)
	if err != nil {
		t.Fatalf("ListZettels failed: %v", err)
	}

	if len(list) != len(zettels) {
		t.Errorf("ListZettels returned %d zettels, want %d", len(list), len(zettels))
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

	count, err := CountZettels(tmpDir)
	if err != nil {
		t.Fatalf("CountZettels failed: %v", err)
	}
	if count != 0 {
		t.Errorf("Empty dir count = %d, want 0", count)
	}

	for i := 0; i < 5; i++ {
		zetID := time.Now().UTC().Add(time.Duration(i) * time.Second).Format("20060102150405")
		err := CreateZettel(tmpDir, zetID, "Test")
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}

	count, err = CountZettels(tmpDir)
	if err != nil {
		t.Fatalf("CountZettels failed: %v", err)
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

	results, err := SearchZettels(tmpDir, "golang")
	if err != nil {
		t.Fatalf("SearchZettels failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("SearchZettels returned %d results, want 2", len(results))
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
		err := CreateZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}

	results, err := SearchZettelTitles(tmpDir, "Golang")
	if err != nil {
		t.Fatalf("SearchZettelTitles failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("SearchZettelTitles returned %d results, want 2", len(results))
	}
}

func TestGetZettelTitle(t *testing.T) {
	tmpDir := t.TempDir()
	zetID := "20231001120000"
	title := "Test Zettel Title"

	err := CreateZettel(tmpDir, zetID, title)
	if err != nil {
		t.Fatalf("CreateZettel failed: %v", err)
	}

	gotTitle, err := GetZettelTitle(tmpDir, zetID)
	if err != nil {
		t.Fatalf("GetZettelTitle failed: %v", err)
	}

	if gotTitle != title {
		t.Errorf("GetZettelTitle = %q, want %q", gotTitle, title)
	}
}

func TestFindTodos(t *testing.T) {
	tmpDir := t.TempDir()

	content := `# Test Zettel

Some content here.

TODO: First todo item
DONE: Completed item
TASK: Second todo item

More content.
`

	zetID := "20231001120000"
	zetPath := filepath.Join(tmpDir, zetID)
	os.MkdirAll(zetPath, 0755)
	os.WriteFile(filepath.Join(zetPath, "README.md"), []byte(content), 0644)

	results, err := FindTodos(tmpDir)
	if err != nil {
		t.Fatalf("FindTodos failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("FindTodos returned %d results, want 2", len(results))
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
		err := CreateZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}

	err := UpdateReadme(tmpDir)
	if err != nil {
		t.Fatalf("UpdateReadme failed: %v", err)
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

func TestIsValidZettelID(t *testing.T) {
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
		got := IsValidZettelID(tt.id)
		if got != tt.valid {
			t.Errorf("IsValidZettelID(%q) = %v, want %v", tt.id, got, tt.valid)
		}
	}
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
		err := CreateZettel(tmpDir, z.id, z.title)
		if err != nil {
			t.Fatalf("Failed to create zettel: %v", err)
		}
	}

	matches, err := FindMatchingZettels(tmpDir, "20231001")
	if err != nil {
		t.Fatalf("FindMatchingZettels failed: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("FindMatchingZettels returned %d matches, want 2", len(matches))
	}

	matches, err = FindMatchingZettels(tmpDir, "202310011200")
	if err != nil {
		t.Fatalf("FindMatchingZettels failed: %v", err)
	}

	if len(matches) != 1 {
		t.Errorf("FindMatchingZettels returned %d matches, want 1", len(matches))
	}

	matches, err = FindMatchingZettels(tmpDir, "20239999")
	if err != nil {
		t.Fatalf("FindMatchingZettels failed: %v", err)
	}

	if len(matches) != 0 {
		t.Errorf("FindMatchingZettels returned %d matches, want 0", len(matches))
	}
}

func TestSearchInFile(t *testing.T) {
	tmpDir := t.TempDir()

	content := `# Test Zettel

This is about golang programming.
More content here.
`

	zetID := "20231001120000"
	zetPath := filepath.Join(tmpDir, zetID)
	os.MkdirAll(zetPath, 0755)
	filePath := filepath.Join(zetPath, "README.md")
	os.WriteFile(filePath, []byte(content), 0644)

	results := SearchInFile(filePath, "golang")
	if len(results) != 1 {
		t.Errorf("SearchInFile returned %d results, want 1", len(results))
	}

	results = SearchInFile(filePath, "nonexistent")
	if len(results) != 0 {
		t.Errorf("SearchInFile returned %d results, want 0", len(results))
	}
}

func TestGenerateZettelID(t *testing.T) {
	id := GenerateZettelID()
	if !IsValidZettelID(id) {
		t.Errorf("GenerateZettelID returned invalid ID: %s", id)
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
