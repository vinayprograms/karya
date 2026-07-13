package task

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/vinayprograms/karya/internal/config"
	"github.com/vinayprograms/karya/internal/jira"
)

var jiraKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]+-\d+$`)
var jiraProjectKeyRe = regexp.MustCompile(`^[A-Z][A-Z0-9]{1,9}$`)

// IsJiraID returns true if the ID matches a JIRA issue key pattern.
func IsJiraID(id string) bool {
	return jiraKeyRe.MatchString(id)
}

// SyncFromJira pulls open JIRA tickets assigned to the user, creates or updates
// corresponding tasks in karya, and handles disappeared tickets.
// Returns the count of issues found in JIRA.
func SyncFromJira(ctx context.Context, cfg *config.Config, client *jira.Client) (int, error) {
	jql := "assignee = currentUser() AND resolution = Unresolved"
	if len(cfg.Jira.ExcludeProjects) > 0 {
		var valid []string
		for _, p := range cfg.Jira.ExcludeProjects {
			if !jiraProjectKeyRe.MatchString(p) {
				return 0, fmt.Errorf("invalid project key in exclude_projects: %q", p)
			}
			valid = append(valid, p)
		}
		jql += " AND project NOT IN (" + strings.Join(valid, ", ") + ")"
	}
	jql += " ORDER BY updated DESC"
	issues, err := client.SearchIssues(ctx, jql)
	if err != nil {
		return 0, fmt.Errorf("searching JIRA: %w", err)
	}

	tasks, err := ListTasks(cfg, "", true)
	if err != nil {
		return 0, fmt.Errorf("loading karya tasks: %w", err)
	}

	// Build map of existing JIRA tasks in karya
	existingJira := make(map[string]*Task)
	for _, t := range tasks {
		if IsJiraID(t.ID) {
			existingJira[t.ID] = t
		}
	}

	// Track which JIRA keys we saw in results
	seen := make(map[string]bool)

	// Build parent key set for sub-task grouping
	issueKeys := make(map[string]*jira.Issue)
	for i := range issues {
		issueKeys[issues[i].Key] = &issues[i]
	}

	for i := range issues {
		issue := &issues[i]
		seen[issue.Key] = true

		// Skip sub-tasks whose parent is also assigned to user (they'll be rendered as children)
		if issue.Fields.Parent != nil {
			if _, parentPresent := issueKeys[issue.Fields.Parent.Key]; parentPresent {
				continue
			}
		}

		if existing, ok := existingJira[issue.Key]; ok {
			if err := updateExistingTask(cfg, existing, issue, issueKeys); err != nil {
				return 0, fmt.Errorf("updating %s: %w", issue.Key, err)
			}
		} else {
			if err := appendNewTask(cfg, issue, issueKeys); err != nil {
				return 0, fmt.Errorf("appending %s: %w", issue.Key, err)
			}
		}
	}

	// Handle disappeared tickets (in karya but not in JIRA results)
	for id, t := range existingJira {
		if seen[id] {
			continue
		}
		// Skip if already completed locally
		if t.IsCompleted(cfg) {
			continue
		}
		if err := handleDisappearedTicket(ctx, cfg, client, t); err != nil {
			return 0, fmt.Errorf("handling disappeared %s: %w", id, err)
		}
	}

	return len(issues), nil
}

func handleDisappearedTicket(ctx context.Context, cfg *config.Config, client *jira.Client, t *Task) error {
	issue, err := client.GetIssue(ctx, t.ID)
	if err != nil {
		return err
	}

	isDone := issue.Fields.Status.StatusCategory.Key == "done"

	// Check if reassigned away
	currentUser, err := client.GetCurrentUser(ctx)
	if err != nil {
		return err
	}
	reassigned := issue.Fields.Assignee == nil || issue.Fields.Assignee.AccountID != currentUser

	if isDone || reassigned {
		if err := UpdateTaskStatus(t, "DONE"); err != nil {
			return err
		}
		if reassigned && issue.Fields.Assignee != nil {
			note := fmt.Sprintf("  Reassigned to %s in JIRA", issue.Fields.Assignee.DisplayName)
			return appendLineAfterTask(t, note)
		}
		if reassigned {
			return appendLineAfterTask(t, "  Unassigned in JIRA")
		}
	}
	return nil
}

func updateExistingTask(cfg *config.Config, t *Task, issue *jira.Issue, allIssues map[string]*jira.Issue) error {
	// Keyword: only overwrite on status category change
	isDone := issue.Fields.Status.StatusCategory.Key == "done"
	currentlyDone := t.IsCompleted(cfg)
	if isDone != currentlyDone {
		newKW := cfg.JiraStatusToKeyword(issue.Fields.Status.Name, isDone)
		if err := UpdateTaskStatus(t, newKW); err != nil {
			return err
		}
	}

	// Update due date
	if issue.Fields.DueDate != "" {
		if t.DueAt != issue.Fields.DueDate {
			if err := setDueDate(t, issue.Fields.DueDate); err != nil {
				return err
			}
		}
	}

	// Update description (replace) and append new comments
	return updateTaskContent(t, issue, allIssues)
}

func appendNewTask(cfg *config.Config, issue *jira.Issue, allIssues map[string]*jira.Issue) error {
	inboxPath := cfg.GetInboxFilePath()

	isDone := issue.Fields.Status.StatusCategory.Key == "done"
	keyword := cfg.JiraStatusToKeyword(issue.Fields.Status.Name, isDone)

	line := renderTaskLine(keyword, issue)
	content := renderTaskBlock(line, issue, allIssues)

	f, err := os.OpenFile(inboxPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString("\n" + content)
	return err
}

func renderTaskLine(keyword string, issue *jira.Issue) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("%s: [%s] %s", keyword, issue.Key, issue.Fields.Summary))

	for _, label := range issue.Fields.Labels {
		sb.WriteString(fmt.Sprintf(" #%s", label))
	}

	if issue.Fields.DueDate != "" {
		sb.WriteString(fmt.Sprintf(" @d:%s", issue.Fields.DueDate))
	}

	// Add parent reference if parent is not in our issue set
	if issue.Fields.Parent != nil {
		sb.WriteString(fmt.Sprintf(" ^%s", issue.Fields.Parent.Key))
	}

	return sb.String()
}

func renderTaskBlock(taskLine string, issue *jira.Issue, allIssues map[string]*jira.Issue) string {
	var sb strings.Builder
	sb.WriteString(taskLine)
	sb.WriteString("\n")

	// Single indented link to the JIRA ticket
	sb.WriteString(fmt.Sprintf("  - %s\n", jiraTicketURL(issue.Key)))

	// Sub-tasks (only those assigned to user that are in our issue set)
	for _, sub := range issue.Fields.Subtasks {
		if subIssue, ok := allIssues[sub.Key]; ok {
			subLine := "  " + renderTaskLine("TODO", subIssue)
			sb.WriteString(subLine + "\n")
			sb.WriteString(fmt.Sprintf("    - %s\n", jiraTicketURL(sub.Key)))
		}
	}

	return sb.String()
}

func jiraTicketURL(key string) string {
	// Extract project prefix to construct URL
	// Standard Atlassian Cloud URL pattern
	return fmt.Sprintf("https://justworks.atlassian.net/browse/%s", key)
}

func updateTaskContent(t *Task, issue *jira.Issue, allIssues map[string]*jira.Issue) error {
	if t.FilePath == "" || t.LineNum == 0 {
		return nil
	}

	data, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if t.LineNum > len(lines) {
		return nil
	}

	// Check if the URL link already exists in the indented block — if so, nothing to do
	ticketURL := jiraTicketURL(issue.Key)
	for i := t.LineNum; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			continue
		}
		if len(line) > 0 && (line[0] != ' ' && line[0] != '\t') {
			break
		}
		if strings.Contains(line, ticketURL) {
			return nil
		}
	}

	// Find the extent of the current indented block
	blockEnd := t.LineNum
	for i := t.LineNum; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			blockEnd = i + 1
			continue
		}
		if len(line) > 0 && (line[0] == ' ' || line[0] == '\t') {
			blockEnd = i + 1
		} else {
			break
		}
	}

	// Replace with just the ticket URL
	finalBlock := buildFinalBlock(issue)

	// Replace the old block
	result := make([]string, 0, len(lines))
	result = append(result, lines[:t.LineNum]...)
	result = append(result, finalBlock...)
	result = append(result, lines[blockEnd:]...)

	return os.WriteFile(t.FilePath, []byte(strings.Join(result, "\n")), 0644)
}

func buildFinalBlock(issue *jira.Issue) []string {
	return []string{fmt.Sprintf("  - %s", jiraTicketURL(issue.Key))}
}


func setDueDate(t *Task, date string) error {
	if t.FilePath == "" || t.LineNum == 0 {
		return nil
	}

	data, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}
	lines := strings.Split(string(data), "\n")
	if t.LineNum > len(lines) {
		return nil
	}

	line := lines[t.LineNum-1]

	// Replace existing @d: or append
	dueRe := regexp.MustCompile(`@d:[^ ]+`)
	if dueRe.MatchString(line) {
		line = dueRe.ReplaceAllString(line, "@d:"+date)
	} else {
		line = line + " @d:" + date
	}

	lines[t.LineNum-1] = line
	return os.WriteFile(t.FilePath, []byte(strings.Join(lines, "\n")), 0644)
}

func appendLineAfterTask(t *Task, text string) error {
	if t.FilePath == "" || t.LineNum == 0 {
		return nil
	}

	data, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")

	// Find end of task's indented block
	insertAt := t.LineNum // after the task line (0-indexed)
	for i := t.LineNum; i < len(lines); i++ {
		line := lines[i]
		if line == "" {
			insertAt = i + 1
			continue
		}
		_, level := StripLinePrefix(line)
		if level > t.IndentLevel {
			insertAt = i + 1
		} else {
			break
		}
	}

	// Insert the new line
	result := make([]string, 0, len(lines)+1)
	result = append(result, lines[:insertAt]...)
	result = append(result, text)
	result = append(result, lines[insertAt:]...)

	return os.WriteFile(t.FilePath, []byte(strings.Join(result, "\n")), 0644)
}

// UpdateTaskLabels updates the tags on a task's line to match JIRA labels.
func UpdateTaskLabels(t *Task, labels []string) error {
	if t.FilePath == "" || t.LineNum == 0 {
		return nil
	}

	data, err := os.ReadFile(t.FilePath)
	if err != nil {
		return err
	}

	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	var lines []string
	lineNum := 0
	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		if lineNum == t.LineNum {
			// Remove existing tags
			tagRe := regexp.MustCompile(`\s*#[^ ]+`)
			line = tagRe.ReplaceAllString(line, "")
			// Append new tags
			for _, label := range labels {
				line += " #" + label
			}
		}
		lines = append(lines, line)
	}

	return os.WriteFile(t.FilePath, []byte(strings.Join(lines, "\n")), 0644)
}
