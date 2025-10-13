package prompt

import (
    "fmt"
    "os"
    "path/filepath"
    "sort"
    "strings"
    "text/template"
)

// BuildSystemPrompt creates the shared system prompt used by all providers.
func BuildSystemPrompt(files []string, context map[string]string) string {
    // Prepare view model
    additional := make([]string, 0, len(context))
    for k, v := range context {
        v = strings.TrimSpace(v)
        if v == "" {
            continue
        }
        switch k {
        case "issue_title", "issue_body":
            continue
        default:
            additional = append(additional, fmt.Sprintf("- %s: %s", k, v))
        }
    }

    // Render from template file if present; fallback to built-in default
    data := map[string]any{
        "FileList":           strings.Join(files, "\n- "),
        "HasAdditionalContext": len(additional) > 0,
        "AdditionalContext":  strings.Join(additional, "\n"),
    }

    // Attempt to load external template for easy A/B and maintenance
    if out, ok := renderTemplateFile("templates/prompt/system.tmpl", data); ok {
        return out
    }
    return renderTemplateString(defaultSystemTemplate, data)
}

// BuildUserPrompt creates the user prompt with task instructions.
func BuildUserPrompt(taskPrompt string) string {
    data := map[string]any{"Task": taskPrompt}
    if out, ok := renderTemplateFile("templates/prompt/user.tmpl", data); ok {
        return out
    }
    return renderTemplateString(defaultUserTemplate, data)
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
    // Conservative ignore set to reduce prompt bloat
    ignoreDirs := map[string]struct{}{
        ".git": {}, "node_modules": {}, "vendor": {}, "dist": {}, "build": {},
        ".next": {}, ".venv": {}, "target": {}, "tmp": {}, ".cache": {},
    }
    const maxFiles = 400
    var files []string

    err := filepath.Walk(repoPath, func(path string, info os.FileInfo, err error) error {
        if err != nil {
            return err
        }

        name := info.Name()

        // Skip ignored directories early
        if info.IsDir() {
            if _, ok := ignoreDirs[name]; ok {
                return filepath.SkipDir
            }
            // Only skip the VCS dir; allow other dotdirs like .github
            if name == ".git" {
                return filepath.SkipDir
            }
            return nil
        }

        // Skip hidden files
        if strings.HasPrefix(name, ".") {
            return nil
        }

        // Skip very large files (>1MB) to avoid token blowups
        if info.Size() > 1<<20 { // 1 MiB
            return nil
        }

        relPath, err := filepath.Rel(repoPath, path)
        if err != nil {
            return err
        }
        files = append(files, relPath)
        return nil
    })
    if err != nil {
        return nil, err
    }

    sort.Strings(files)
    if len(files) > maxFiles {
        files = files[:maxFiles]
    }
    return files, nil
}

// --- template rendering helpers and defaults ---

func renderTemplateFile(path string, data any) (string, bool) {
    b, err := os.ReadFile(path)
    if err != nil {
        return "", false
    }
    return renderTemplateString(string(b), data), true
}

func renderTemplateString(tmpl string, data any) string {
    t := template.Must(template.New("prompt").Parse(tmpl))
    var sb strings.Builder
    _ = t.Execute(&sb, data)
    return sb.String()
}

const defaultSystemTemplate = `You are a code modification assistant working on a GitHub repository.

Repository structure:
- {{.FileList}}

{{- if .HasAdditionalContext }}
Additional Context:
{{.AdditionalContext}}
{{ end }}

When making changes:
1. Understand the task thoroughly before making modifications
2. Make minimal, focused changes that address the specific request
3. Preserve existing code style and conventions
4. Include complete file content in your response (not just diffs)
5. Do not execute unknown scripts or perform destructive operations

## PR Size Best Practices

Small PRs are preferred and more likely to be merged quickly.

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

Note: The system may automatically split large changes into multiple PRs.
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

const defaultUserTemplate = `Task: {{.Task}}

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

Make sure to include the COMPLETE file content when providing code changes, not just the changes.`
