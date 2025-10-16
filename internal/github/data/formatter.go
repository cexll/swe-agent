package data

import (
	"fmt"
	"strings"

	gh "github.com/cexll/swe/internal/github"
)

// formatContext returns a single-paragraph summary about the PR or Issue.
func formatContext(contextData interface{}, isPR bool) string {
	if isPR {
		pr := contextData.(PullRequest)
		return fmt.Sprintf(
			"PR Title: %s\nPR Author: %s\nPR Branch: %s -> %s\nPR State: %s\nPR Additions: %d\nPR Deletions: %d\nTotal Commits: %d\nChanged Files: %d files",
			pr.Title,
			pr.Author.Login,
			pr.HeadRefName,
			pr.BaseRefName,
			pr.State,
			pr.Additions,
			pr.Deletions,
			pr.Commits.TotalCount,
			len(pr.Files.Nodes),
		)
	}
	is := contextData.(Issue)
	return fmt.Sprintf(
		"Issue Title: %s\nIssue Author: %s\nIssue State: %s",
		is.Title, is.Author.Login, is.State,
	)
}

// formatBody sanitizes and optionally rewrites image URLs (if map provided).
func formatBody(body string, imageURLMap map[string]string) string {
	processed := body
	for orig, local := range imageURLMap {
		processed = strings.ReplaceAll(processed, orig, local)
	}
	return gh.SanitizeContent(processed)
}

// formatComments renders comments as author/timestamp+sanitized body pairs.
func formatComments(comments []Comment, imageURLMap map[string]string) string {
	var out []string
	for _, c := range comments {
		if c.IsMinimized {
			continue
		}
		body := c.Body
		for orig, local := range imageURLMap {
			body = strings.ReplaceAll(body, orig, local)
		}
		body = gh.SanitizeContent(body)
		out = append(out, fmt.Sprintf("[%s at %s]: %s", c.Author.Login, c.CreatedAt, body))
	}
	return strings.Join(out, "\n\n")
}

// formatReviewComments renders review summaries and inline comments.
func formatReviewComments(reviews *struct{ Nodes []Review }, imageURLMap map[string]string) string {
	if reviews == nil || len(reviews.Nodes) == 0 {
		return ""
	}
	blocks := make([]string, 0, len(reviews.Nodes))
	for _, r := range reviews.Nodes {
		header := fmt.Sprintf("[Review by %s at %s]: %s", r.Author.Login, r.SubmittedAt, r.State)
		var b strings.Builder
		b.WriteString(header)
		if strings.TrimSpace(r.Body) != "" {
			body := r.Body
			for orig, local := range imageURLMap {
				body = strings.ReplaceAll(body, orig, local)
			}
			b.WriteString("\n")
			b.WriteString(gh.SanitizeContent(body))
		}
		if len(r.Comments.Nodes) > 0 {
			for _, c := range r.Comments.Nodes {
				if c.IsMinimized {
					continue
				}
				body := c.Body
				for orig, local := range imageURLMap {
					body = strings.ReplaceAll(body, orig, local)
				}
				body = gh.SanitizeContent(body)
				line := "?"
				if c.Line != nil {
					line = fmt.Sprintf("%d", *c.Line)
				}
				b.WriteString("\n")
				b.WriteString(fmt.Sprintf("  [Comment on %s:%s]: %s", c.Path, line, body))
			}
		}
		blocks = append(blocks, b.String())
	}
	return strings.Join(blocks, "\n\n")
}

// formatChangedFiles renders changed files list.
func formatChangedFiles(files []File) string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, fmt.Sprintf("- %s (%s) +%d/-%d", f.Path, f.ChangeType, f.Additions, f.Deletions))
	}
	return strings.Join(out, "\n")
}

// formatChangedFilesWithSHA renders changed files with SHA suffix.
func formatChangedFilesWithSHA(files []GitHubFileWithSHA) string {
	out := make([]string, 0, len(files))
	for _, f := range files {
		out = append(out, fmt.Sprintf("- %s (%s) +%d/-%d SHA: %s", f.Path, f.ChangeType, f.Additions, f.Deletions, f.SHA))
	}
	return strings.Join(out, "\n")
}

// GenerateXMLParams controls XML prompt generation analogous
type GenerateXMLParams struct {
	Repository         string
	IsPR               bool
	Number             int
	EventType          string
	TriggerContext     string
	TriggerUsername    string
	TriggerDisplayName string
	TriggerPhrase      string
	TriggerComment     string // optional
	ClaudeCommentID    string
	BaseBranch         string

	// Data
	ContextData         interface{}
	Comments            []Comment
	ReviewData          *struct{ Nodes []Review }
	ChangedFilesWithSHA []GitHubFileWithSHA
	ImageURLMap         map[string]string
}

// GenerateXML builds the XML-tagged prompt sections similar to create-prompt/index.ts.
func GenerateXML(p GenerateXMLParams) string {
	formattedContext := formatContext(p.ContextData, p.IsPR)
	formattedComments := formatComments(p.Comments, p.ImageURLMap)
	formattedReview := ""
	formattedChanged := ""
	if p.IsPR {
		formattedReview = formatReviewComments(p.ReviewData, p.ImageURLMap)
		formattedChanged = formatChangedFilesWithSHA(p.ChangedFilesWithSHA)
	}
	bodyText := "No description provided"
	switch v := p.ContextData.(type) {
	case PullRequest:
		if strings.TrimSpace(v.Body) != "" {
			bodyText = formatBody(v.Body, p.ImageURLMap)
		}
	case Issue:
		if strings.TrimSpace(v.Body) != "" {
			bodyText = formatBody(v.Body, p.ImageURLMap)
		}
	}

	var b strings.Builder
	b.WriteString("<formatted_context>\n")
	b.WriteString(formattedContext)
	b.WriteString("\n</formatted_context>\n\n")

	b.WriteString("<pr_or_issue_body>\n")
	b.WriteString(bodyText)
	b.WriteString("\n</pr_or_issue_body>\n\n")

	b.WriteString("<comments>\n")
	if formattedComments != "" {
		b.WriteString(formattedComments)
	} else {
		b.WriteString("No comments")
	}
	b.WriteString("\n</comments>\n\n")

	if p.IsPR {
		b.WriteString("<review_comments>\n")
		if formattedReview != "" {
			b.WriteString(formattedReview)
		} else {
			b.WriteString("No review comments")
		}
		b.WriteString("\n</review_comments>\n\n")

		b.WriteString("<changed_files>\n")
		if formattedChanged != "" {
			b.WriteString(formattedChanged)
		} else {
			b.WriteString("No files changed")
		}
		b.WriteString("\n</changed_files>\n\n")
	}

	b.WriteString(fmt.Sprintf("<event_type>%s</event_type>\n", p.EventType))
	if p.IsPR {
		b.WriteString("<is_pr>true</is_pr>\n")
	} else {
		b.WriteString("<is_pr>false</is_pr>\n")
	}
	b.WriteString("<trigger_context>")
	b.WriteString(p.TriggerContext)
	b.WriteString("</trigger_context>\n")
	b.WriteString("<repository>")
	b.WriteString(p.Repository)
	b.WriteString("</repository>\n")
	if p.IsPR {
		b.WriteString(fmt.Sprintf("<pr_number>%d</pr_number>\n", p.Number))
	} else {
		b.WriteString(fmt.Sprintf("<issue_number>%d</issue_number>\n", p.Number))
	}
	b.WriteString("<claude_comment_id>")
	b.WriteString(p.ClaudeCommentID)
	b.WriteString("</claude_comment_id>\n")
	b.WriteString("<trigger_username>")
	if p.TriggerUsername != "" {
		b.WriteString(p.TriggerUsername)
	} else {
		b.WriteString("Unknown")
	}
	b.WriteString("</trigger_username>\n")
	b.WriteString("<trigger_display_name>")
	if p.TriggerDisplayName != "" {
		b.WriteString(p.TriggerDisplayName)
	} else if p.TriggerUsername != "" {
		b.WriteString(p.TriggerUsername)
	} else {
		b.WriteString("Unknown")
	}
	b.WriteString("</trigger_display_name>\n")
	b.WriteString("<trigger_phrase>")
	b.WriteString(p.TriggerPhrase)
	b.WriteString("</trigger_phrase>\n")
	if strings.TrimSpace(p.TriggerComment) != "" {
		b.WriteString("<trigger_comment>\n")
		b.WriteString(gh.SanitizeContent(p.TriggerComment))
		b.WriteString("\n</trigger_comment>\n")
	}
	return b.String()
}
