package prompt

import (
    "fmt"
    "os"
    "path/filepath"
    "strings"
)

// BuildSystemPrompt creates the shared system prompt used by all providers.
func BuildSystemPrompt(files []string, context map[string]string) string {
    fileList := strings.Join(files, "\n- ")

    prompt := fmt.Sprintf(`You are a code modification assistant working on a GitHub repository.

Repository structure:
- %s

`, fileList)

    // Add context if available (excluding issue content which is already part of the main prompt)
    additionalContext := make([]string, 0, len(context))
    for key, value := range context {
        trimmedValue := strings.TrimSpace(value)
        if trimmedValue == "" {
            continue
        }
        switch key {
        case "issue_title", "issue_body":
            // Skip, these are part of user prompt already
            continue
        default:
            additionalContext = append(additionalContext, fmt.Sprintf("- %s: %s", key, trimmedValue))
        }
    }

    if len(additionalContext) > 0 {
        prompt += "\nAdditional Context:\n"
        prompt += strings.Join(additionalContext, "\n")
        prompt += "\n"
    }

    prompt += `
When making changes:
1. Understand the task thoroughly before making modifications
2. Make minimal, focused changes that address the specific request
3. Preserve existing code style and conventions
4. Include complete file content in your response (not just diffs)

## PR Size Best Practices

**Small PRs are preferred and more likely to be merged quickly.**

If you need to modify more than 8 files or 300 lines:
- Consider splitting the work into multiple logical PRs
- Separate independent changes:
  * Tests can be added in a separate PR
  * Documentation updates can be independent
  * Infrastructure/internal changes separate from core logic
  * Command-line interface changes separate from core

Example split strategy:
- PR 1: Add test infrastructure
- PR 2: Update documentation
- PR 3: Implement core functionality
- PR 4: Update CLI

**Note:** The system will automatically split large changes into multiple PRs.
Focus on making logical, atomic changes that are easy to review.

Return your changes in this exact format:
<file path="path/to/file">
<content>
... complete file content ...
</content>
</file>

<summary>
Brief description of what was changed
</summary>`

    return prompt
}

// BuildUserPrompt creates the user prompt with task instructions.
func BuildUserPrompt(taskPrompt string) string {
    return fmt.Sprintf(`Task: %s

You can choose to either:

1. Provide code changes (if modifications are needed):
<file path="path/to/file.ext">
<content>
... full file content here ...
</content>
</file>

<summary>
Brief description of changes made
</summary>

2. Provide analysis/answer only (if no code changes needed):
<summary>
Your analysis, recommendations, or answer here.
You can include explanations, task lists, or any helpful information.
</summary>

Make sure to include the COMPLETE file content when providing code changes, not just the changes.`, taskPrompt)
}

// BuildFullPrompt composes system + user prompts, with optional prefix (e.g., for Codex execution hint).
func BuildFullPrompt(taskPrompt string, files []string, context map[string]string, prefix string) string {
    system := BuildSystemPrompt(files, context)
    user := BuildUserPrompt(taskPrompt)
    if prefix != "" {
        return prefix + system + "\n\n" + user
    }
    return fmt.Sprintf("System: %s\n\nUser: %s", system, user)
}

// ListRepoFiles lists all files in the repository (excluding .git and hidden files/directories).
func ListRepoFiles(repoPath string) ([]string, error) {
    var files []string

    err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        // Skip .git directory
        if info.IsDir() && info.Name() == ".git" {
            return filepath.SkipDir
        }

        // Skip directories and hidden files
        if info.IsDir() || strings.HasPrefix(info.Name(), ".") {
            return nil
        }

        // Get relative path
        relPath, err := filepath.Rel(repoPath, path)
        if err != nil {
            return err
        }

        files = append(files, relPath)
        return nil
    })

    return files, err
}

