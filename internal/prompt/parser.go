package prompt

import (
    "log"
    "os"
    "regexp"
    "strings"
)

// ParsedFile represents a parsed file change block from a model response.
type ParsedFile struct {
    Path    string
    Content string
}

// ParseResponse parses an LLM response and returns extracted file changes and a summary.
// It supports both XML-style <file path="..."><content>...</content></file> blocks
// and Markdown-style code blocks as a fallback. The summary is extracted from
// <summary>...</summary> tags, markdown headers (## Summary), or defaults.
func ParseResponse(response string) ([]ParsedFile, string, error) {
    files := make([]ParsedFile, 0)

    if os.Getenv("DEBUG_CLAUDE_PARSING") == "true" {
        log.Printf("[ParseResponse] input length=%d", len(response))
    }

    // Primary: XML blocks
    files = append(files, parseXMLFileBlocks(response)...)

    // Fallback: Markdown code blocks
    if len(files) == 0 {
        files = append(files, parseMarkdownCodeBlocks(response)...)
    }

    // Summary extraction
    summary := extractSummary(response, len(files) > 0)

    if len(files) == 0 && strings.TrimSpace(summary) == "" {
        return nil, "", ErrNoContent
    }

    return files, summary, nil
}

// ErrNoContent indicates the response did not contain any files or summary content.
var ErrNoContent = &parseError{"no content found in response"}

type parseError struct{ msg string }

func (e *parseError) Error() string { return e.msg }

// --- helpers (mirrored from the more robust Claude implementation) ---

func parseXMLFileBlocks(response string) []ParsedFile {
    var files []ParsedFile
    fileRegex := regexp.MustCompile(`(?s)<file\s+path=["']([^"']+)["']>\s*<content>\s*(.*?)\s*</content>\s*</file>`)
    matches := fileRegex.FindAllStringSubmatch(response, -1)
    for _, m := range matches {
        if len(m) >= 3 {
            path := strings.TrimSpace(m[1])
            content := m[2] // preserve whitespace
            if path != "" {
                files = append(files, ParsedFile{Path: path, Content: content})
            }
        }
    }
    return files
}

func parseMarkdownCodeBlocks(response string) []ParsedFile {
    var files []ParsedFile

    // Pattern 1: ```lang path
    codeBlockRegex1 := regexp.MustCompile("```(\\w+)\\s+([^\\s\\n]*[./][^\\s\\n]*)\\s*\\n([\\s\\S]*?)\\n```")
    m1 := codeBlockRegex1.FindAllStringSubmatch(response, -1)
    for _, m := range m1 {
        if len(m) >= 4 {
            path := strings.TrimSpace(m[2])
            content := m[3]
            files = append(files, ParsedFile{Path: path, Content: content})
        }
    }

    // Pattern 2: **filename:** followed by fenced code block
    headerRegex := regexp.MustCompile(`(?s)\*\*([^*]+)\*\*:?\s*\n` + "`" + `{3}\w*\s*\n(.*?)\n` + "`" + `{3}`)
    m2 := headerRegex.FindAllStringSubmatch(response, -1)
    for _, m := range m2 {
        if len(m) >= 3 {
            path := strings.TrimSpace(m[1])
            path = strings.TrimSuffix(path, ":")
            content := m[2]
            if strings.Contains(path, ".") || strings.Contains(path, "/") {
                files = append(files, ParsedFile{Path: path, Content: content})
            }
        }
    }

    return files
}

func extractSummary(response string, hasFiles bool) string {
    // Prefer explicit summary tags
    summaryRegex := regexp.MustCompile(`(?s)<summary>\s*(.*?)\s*</summary>`)
    if m := summaryRegex.FindStringSubmatch(response); len(m) >= 2 {
        return strings.TrimSpace(m[1])
    }

    // Markdown header fallback
    headerRegex := regexp.MustCompile(`(?s)#+\s*Summary\s*\n(.*?)(?:\n#+|$)`)
    if m := headerRegex.FindStringSubmatch(response); len(m) >= 2 {
        return strings.TrimSpace(m[1])
    }

    if !hasFiles {
        return strings.TrimSpace(response)
    }
    return "Code changes applied"
}

