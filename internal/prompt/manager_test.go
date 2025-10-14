package prompt

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestManager_ListRepoFiles(t *testing.T) {
	t.Helper()

	tmpDir := t.TempDir()

	mustWrite := func(rel, content string) {
		full := filepath.Join(tmpDir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("failed to create directory for %s: %v", rel, err)
		}
		if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
			t.Fatalf("failed to write %s: %v", rel, err)
		}
	}

	// Visible files
	mustWrite("main.go", "package main")
	mustWrite("nested/util.go", "package nested")

	// Hidden files and git metadata should be ignored
	mustWrite(".hidden", "secret")
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0o755); err != nil {
		t.Fatalf("failed to create git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "config"), []byte(""), 0o644); err != nil {
		t.Fatalf("failed to write git config: %v", err)
	}

	files, err := NewManager().ListRepoFiles(tmpDir)
	if err != nil {
		t.Fatalf("ListRepoFiles returned error: %v", err)
	}

	sort.Strings(files)
	want := []string{"main.go", "nested/util.go"}
	if len(files) != len(want) {
		t.Fatalf("ListRepoFiles = %v, want %v", files, want)
	}
	for i, file := range want {
		if files[i] != file {
			t.Fatalf("ListRepoFiles[%d] = %s, want %s", i, files[i], file)
		}
	}
}

func TestManager_BuildCommentMetadata_SanitizesAndDefaults(t *testing.T) {
	manager := NewManager()
	context := map[string]string{
		"issue_title":      "<b>Fix login</b>",
		"issue_body":       "<script>alert('x')</script>",
		"trigger_context":  "<div>Manual</div>",
		"trigger_phrase":   "<assistant>",
		"is_pr":            "TrUe",
		"pr_number":        "42",
		"issue_number":     "101",
		"trigger_username": "",
	}

	metadata := manager.BuildCommentMetadata(context)

	if metadata.Repository != "local repository" {
		t.Fatalf("Repository = %q, want local repository", metadata.Repository)
	}
	if metadata.IssueTitle != "&lt;b&gt;Fix login&lt;/b&gt;" {
		t.Fatalf("IssueTitle = %q, want sanitized title", metadata.IssueTitle)
	}
	if metadata.IssueBody != "&lt;script&gt;alert(&#39;x&#39;)&lt;/script&gt;" {
		t.Fatalf("IssueBody = %q, want sanitized body", metadata.IssueBody)
	}
	if metadata.TriggerContext != "&lt;div&gt;Manual&lt;/div&gt;" {
		t.Fatalf("TriggerContext = %q, want sanitized context", metadata.TriggerContext)
	}
	if metadata.TriggerPhrase != "&lt;assistant&gt;" {
		t.Fatalf("TriggerPhrase = %q, want sanitized phrase", metadata.TriggerPhrase)
	}
	if !metadata.IsPR {
		t.Fatal("IsPR should be true when context uses TrUe")
	}
	if metadata.PRNumber != "42" || metadata.IssueNumber != "101" {
		t.Fatalf("PRNumber/IssueNumber = %s/%s, want 42/101", metadata.PRNumber, metadata.IssueNumber)
	}
}

func TestManager_BuildInstructionChecklist_PRHints(t *testing.T) {
	manager := NewManager()
	context := map[string]string{
		"is_pr":          "true",
		"base_branch":    "develop",
		"trigger_phrase": "@assistant",
		"event_name":     "pull_request_review_comment",
	}

	checklist := manager.BuildInstructionChecklist(context)

	if !strings.Contains(checklist, "git diff origin/develop...HEAD") {
		t.Fatalf("Checklist missing diff guidance:\n%s", checklist)
	}
	if !strings.Contains(checklist, "origin/develop") {
		t.Fatalf("Checklist missing base branch reference:\n%s", checklist)
	}
	if !strings.Contains(checklist, "@assistant") {
		t.Fatalf("Checklist missing trigger phrase reference:\n%s", checklist)
	}
}

func TestManager_BuildDefaultSystemPrompt_IncludesTriggerComment(t *testing.T) {
	manager := NewManager()
	files := []string{"main.go", "dir/helper.go"}
	context := map[string]string{
		"issue_body":       "<b>Fix</b> vulnerability",
		"trigger_comment":  "<i>Implement soon</i>",
		"trigger_phrase":   "@assistant",
		"event_name":       "pull_request_review_comment",
		"is_pr":            "true",
		"base_branch":      "develop",
		"claude_branch":    "feature/login",
		"repository":       "owner/repo",
		"trigger_username": "dev",
	}

	output := manager.BuildDefaultSystemPrompt(files, context)

	if !strings.Contains(output, "<formatted_context>") {
		t.Fatalf("Expected formatted context section:\n%s", output)
	}
	if !strings.Contains(output, "&lt;b&gt;Fix&lt;/b&gt; vulnerability") {
		t.Fatalf("Issue body should be sanitized:\n%s", output)
	}
	if !strings.Contains(output, "<trigger_comment>\n&lt;i&gt;Implement soon&lt;/i&gt;\n</trigger_comment>") {
		t.Fatalf("Trigger comment should be sanitized and included:\n%s", output)
	}
	if !strings.Contains(output, "git diff origin/develop...HEAD") {
		t.Fatalf("PR guidance missing base branch diff hint:\n%s", output)
	}
	if !strings.Contains(output, "feature/login") {
		t.Fatalf("Expected branch name instructions:\n%s", output)
	}
}

func TestManager_BuildCommitPrompt_ContainsSections(t *testing.T) {
	manager := NewManager()
	files := []string{"main.go"}
	context := map[string]string{
		"issue_body":           "Body",
		"trigger_username":     "alice",
		"trigger_display_name": "Alice Dev",
		"base_branch":          "develop",
		"claude_branch":        "feature/login",
	}

	output := manager.BuildCommitPrompt(files, context)

	contains := func(substr string) {
		if !strings.Contains(output, substr) {
			t.Fatalf("Commit prompt missing %q:\n%s", substr, output)
		}
	}

	contains("<commit_message>")
	contains("Concise imperative subject")
	contains("git add <files>")
	contains("feature/login")
	contains("Co-authored-by: Alice Dev <alice@users.noreply.github.com>")
}

func TestFormatRepositoryContext_Additional(t *testing.T) {
	ctx := map[string]string{
		"custom_field": "value",
	}

	result := formatRepositoryContext([]string{"main.go"}, ctx)
	if !strings.Contains(result, "- main.go") {
		t.Fatalf("Expected file listing:\n%s", result)
	}
	if !strings.Contains(result, "Additional context") || !strings.Contains(result, "- custom_field: value") {
		t.Fatalf("Expected additional context section:\n%s", result)
	}
}

func TestGetCommitInstructionsText_Variants(t *testing.T) {
	withSigning := getCommitInstructionsText("develop", "", "alice", "Alice Dev", true)
	if !strings.Contains(withSigning, "Co-authored-by: Alice Dev <alice@users.noreply.github.com>") {
		t.Fatalf("Expected co-author trailer with signing:\n%s", withSigning)
	}
	if !strings.Contains(withSigning, "mcp__github_file_ops__commit_files") {
		t.Fatalf("Expected commit_files guidance when signing:\n%s", withSigning)
	}

	withoutSigning := getCommitInstructionsText("", "", "Unknown", "", false)
	if strings.Contains(withoutSigning, "Co-authored-by") {
		t.Fatalf("Should not include co-author when trigger user unknown:\n%s", withoutSigning)
	}
	if !strings.Contains(withoutSigning, "git push origin the PR branch") {
		t.Fatalf("Expected default branch push guidance:\n%s", withoutSigning)
	}

	withClaudeBranch := getCommitInstructionsText("main", "feature/login", "bob", "Bob", false)
	if !strings.Contains(withClaudeBranch, "git push origin feature/login") {
		t.Fatalf("Expected push to claude branch:\n%s", withClaudeBranch)
	}
}

func TestBranchHelpers(t *testing.T) {
	if defaultBranch("develop") != "develop" {
		t.Fatal("defaultBranch should prefer explicit branch")
	}
	if defaultBranch("") != "main" {
		t.Fatal("defaultBranch should fallback to main")
	}
	if chooseBranchName("feature") != "feature" {
		t.Fatal("chooseBranchName should return provided name")
	}
	if chooseBranchName("") != "the PR branch" {
		t.Fatal("chooseBranchName should fallback label")
	}
}

func TestManager_BuildSystemPrompt_Delegates(t *testing.T) {
	manager := NewManager()
	files := []string{"main.go"}
	context := map[string]string{"trigger_phrase": "/code"}

	defaultPrompt := manager.BuildDefaultSystemPrompt(files, context)
	systemPrompt := manager.BuildSystemPrompt(files, context)

	if systemPrompt != defaultPrompt {
		t.Fatalf("BuildSystemPrompt should delegate to default prompt builder")
	}
}

func TestManager_BuildUserPrompt_Format(t *testing.T) {
	output := Manager{}.BuildUserPrompt("Implement feature X")

	if !strings.Contains(output, "Task: Implement feature X") {
		t.Fatalf("User prompt missing task line:\n%s", output)
	}
	if !strings.Contains(output, "Code changes required (EXAMPLE") {
		t.Fatalf("User prompt missing code change example section:\n%s", output)
	}
	if !strings.Contains(output, "Never return the literal strings \"path/to/file.ext\"") {
		t.Fatalf("User prompt missing placeholder warning:\n%s", output)
	}
	if !strings.Contains(output, "<summary>") {
		t.Fatalf("User prompt missing summary guidance:\n%s", output)
	}
}
