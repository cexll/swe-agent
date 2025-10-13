package prompt

import (
    "fmt"
    "strings"
)

// BuildSystemPrompt creates the system prompt shared by all AI providers.
// It lists the repository structure and appends any additional context
// (excluding issue title/body which are already part of the main prompt).
func BuildSystemPrompt(files []string, context map[string]string) string {
    fileList := strings.Join(files, "\n- ")

    prompt := fmt.Sprintf(`You are a code modification assistant working on a GitHub repository.

Repository structure:
- %s

`, fileList)

    // Append additional context if present (skip issue title/body to avoid duplication)
    additionalContext := make([]string, 0, len(context))
    for key, value := range context {
        trimmedValue := strings.TrimSpace(value)
        if trimmedValue == "" {
            continue
        }
        switch key {
        case "issue_title", "issue_body":
            // Skip, already included in the main user prompt
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