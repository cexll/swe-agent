package shared

import (
	"fmt"
	"log"
	"regexp"
	"strings"
)

// FileChange captures a single file edit extracted from a provider response.
type FileChange struct {
	Path    string
	Content string
}

// ParseResult contains the structured content extracted from a provider response.
type ParseResult struct {
	Files   []FileChange
	Summary string
}

var (
	placeholderPaths = map[string]struct{}{
		"path/to/file.ext":         {},
		"relative/path/to/file.go": {},
	}

	placeholderContentSnippets = []string{
		"... full file content here ...",
		"entire updated file content here",
	}

	placeholderSummaries = map[string]struct{}{
		"brief description of changes made":     {},
		"add user authentication to handler.go": {},
	}

	permissionRequestPhrases = []string{
		"would you like me to proceed",
		"would you like me to continue",
		"shall i proceed",
		"if you grant the necessary permissions",
		"if you grant the necessary permission",
		"if you grant me permission",
		"grant the necessary permissions",
		"prefer to create them manually",
		"let me know if you want me to proceed",
		"i can start implementing once you confirm",
		"i can proceed once you approve",
	}
)

// ParseResponse converts a raw provider response into structured file changes and summary.
// The provider label is used for log prefixes (e.g., "Claude" / "Codex").
func ParseResponse(providerLabel, response string) (*ParseResult, error) {
	result := &ParseResult{
		Files: []FileChange{},
	}

	text := strings.TrimSpace(response)
	if text == "" {
		return nil, fmt.Errorf("no content found in response")
	}

	files := extractXMLFileBlocks(text)
	if len(files) == 0 {
		files = append(files, extractMarkdownFileBlocks(text)...)
	}
	files = filterPlaceholderFiles(providerLabel, files)

	hasFiles := len(files) > 0
	summary := extractSummary(text, hasFiles)

	if hasFiles {
		if isPlaceholderSummary(summary) {
			summary = "Code changes applied"
		}
	} else {
		if isPlaceholderSummary(summary) {
			return nil, fmt.Errorf("placeholder summary detected in response")
		}

		if strings.TrimSpace(summary) == "" {
			return nil, fmt.Errorf("no content found in response")
		}

		if ContainsPermissionRequest(summary) {
			return nil, fmt.Errorf("permission request detected in response")
		}
	}

	result.Files = files
	result.Summary = summary
	return result, nil
}

// ContainsPermissionRequest reports whether the text asks the user for permission before proceeding.
func ContainsPermissionRequest(text string) bool {
	s := strings.ToLower(text)
	for _, phrase := range permissionRequestPhrases {
		if strings.Contains(s, phrase) {
			return true
		}
	}
	return false
}

func extractXMLFileBlocks(response string) []FileChange {
	var files []FileChange

	fileRegex := regexp.MustCompile(`(?s)<file\s+path=["']([^"']+)["']>\s*<content>\s*(.*?)\s*</content>\s*</file>`)
	fileMatches := fileRegex.FindAllStringSubmatch(response, -1)

	for _, match := range fileMatches {
		if len(match) >= 3 {
			path := strings.TrimSpace(match[1])
			content := match[2]

			if path != "" {
				files = append(files, FileChange{
					Path:    path,
					Content: content,
				})
			}
		}
	}

	return files
}

func extractMarkdownFileBlocks(response string) []FileChange {
	var files []FileChange

	codeBlockRegex1 := regexp.MustCompile("```(\\w+)\\s+([^\\s\\n]*[./][^\\s\\n]*)\\s*\\n([\\s\\S]*?)\\n```")
	matches1 := codeBlockRegex1.FindAllStringSubmatch(response, -1)

	for _, match := range matches1 {
		if len(match) >= 4 {
			path := strings.TrimSpace(match[2])
			content := match[3]

			if path != "" {
				files = append(files, FileChange{
					Path:    path,
					Content: content,
				})
			}
		}
	}

	headerRegex := regexp.MustCompile(`(?s)\*\*([^*]+)\*\*:?\s*\n` + "`" + `{3}\w*\s*\n(.*?)\n` + "`" + `{3}`)
	matches2 := headerRegex.FindAllStringSubmatch(response, -1)

	for _, match := range matches2 {
		if len(match) >= 3 {
			path := strings.TrimSpace(match[1])
			path = strings.TrimSuffix(path, ":")
			content := match[2]

			if path != "" && (strings.Contains(path, ".") || strings.Contains(path, "/")) {
				files = append(files, FileChange{
					Path:    path,
					Content: content,
				})
			}
		}
	}

	return files
}

func isPlaceholderPath(path string) bool {
	key := strings.ToLower(strings.TrimSpace(path))
	_, ok := placeholderPaths[key]
	return ok
}

func isPlaceholderContent(content string) bool {
	for _, snippet := range placeholderContentSnippets {
		if strings.Contains(content, snippet) {
			return true
		}
	}
	return false
}

func filterPlaceholderFiles(providerLabel string, files []FileChange) []FileChange {
	var filtered []FileChange

	for _, file := range files {
		if isPlaceholderPath(file.Path) {
			logPlaceholder(providerLabel, "Ignoring placeholder file path entry: %s", file.Path)
			continue
		}

		if isPlaceholderContent(file.Content) {
			logPlaceholder(providerLabel, "Ignoring placeholder file content for path: %s", file.Path)
			continue
		}

		filtered = append(filtered, file)
	}

	return filtered
}

func extractSummary(response string, hasFiles bool) string {
	summaryRegex := regexp.MustCompile(`(?s)<summary>\s*(.*?)\s*</summary>`)
	summaryMatch := summaryRegex.FindStringSubmatch(response)
	if len(summaryMatch) >= 2 {
		return strings.TrimSpace(summaryMatch[1])
	}

	headerRegex := regexp.MustCompile(`(?s)#+\s*Summary\s*\n(.*?)(?:\n#+|$)`)
	headerMatch := headerRegex.FindStringSubmatch(response)
	if len(headerMatch) >= 2 {
		return strings.TrimSpace(headerMatch[1])
	}

	if !hasFiles {
		return strings.TrimSpace(response)
	}

	return "Code changes applied"
}

func isPlaceholderSummary(summary string) bool {
	if summary == "" {
		return false
	}
	_, ok := placeholderSummaries[strings.ToLower(strings.TrimSpace(summary))]
	return ok
}

// IsPlaceholderSummary exposes placeholder summary detection for tests and callers that
// need to perform additional validation.
func IsPlaceholderSummary(summary string) bool {
	return isPlaceholderSummary(summary)
}

func logPlaceholder(providerLabel, format string, args ...interface{}) {
	if providerLabel == "" {
		log.Printf(format, args...)
		return
	}
	log.Printf("["+providerLabel+"] "+format, args...)
}
