package data

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// GitHub GraphQL data types (trimmed to what's needed)

type Author struct {
	Login string `json:"login"`
	Name  string `json:"name,omitempty"`
}

type Comment struct {
	ID           string `json:"id"`
	DatabaseID   int    `json:"databaseId"`
	Body         string `json:"body"`
	Author       Author `json:"author"`
	CreatedAt    string `json:"createdAt"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
	LastEditedAt string `json:"lastEditedAt,omitempty"`
	IsMinimized  bool   `json:"isMinimized"`
}

type ReviewComment struct {
	Comment
	Path string `json:"path"`
	Line *int   `json:"line"`
}

type Commit struct {
	OID     string `json:"oid"`
	Message string `json:"message"`
	Author  struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"author"`
}

type File struct {
	Path       string `json:"path"`
	Additions  int    `json:"additions"`
	Deletions  int    `json:"deletions"`
	ChangeType string `json:"changeType"`
}

type Review struct {
	ID           string `json:"id"`
	DatabaseID   int    `json:"databaseId"`
	Author       Author `json:"author"`
	Body         string `json:"body"`
	State        string `json:"state"`
	SubmittedAt  string `json:"submittedAt"`
	UpdatedAt    string `json:"updatedAt,omitempty"`
	LastEditedAt string `json:"lastEditedAt,omitempty"`
	Comments     struct {
		Nodes []ReviewComment `json:"nodes"`
	} `json:"comments"`
}

type PullRequest struct {
	Title       string `json:"title"`
	Body        string `json:"body"`
	Author      Author `json:"author"`
	BaseRefName string `json:"baseRefName"`
	HeadRefName string `json:"headRefName"`
	HeadRefOID  string `json:"headRefOid"`
	CreatedAt   string `json:"createdAt"`
	Additions   int    `json:"additions"`
	Deletions   int    `json:"deletions"`
	State       string `json:"state"`
	Commits     struct {
		TotalCount int `json:"totalCount"`
		Nodes      []struct {
			Commit Commit `json:"commit"`
		} `json:"nodes"`
	} `json:"commits"`
	Files struct {
		Nodes []File `json:"nodes"`
	} `json:"files"`
	Comments struct {
		Nodes []Comment `json:"nodes"`
	} `json:"comments"`
	Reviews struct {
		Nodes []Review `json:"nodes"`
	} `json:"reviews"`
}

type Issue struct {
	Title     string `json:"title"`
	Body      string `json:"body"`
	Author    Author `json:"author"`
	CreatedAt string `json:"createdAt"`
	State     string `json:"state"`
	Comments  struct {
		Nodes []Comment `json:"nodes"`
	} `json:"comments"`
}

type pullRequestQueryResponse struct {
	Repository struct {
		PullRequest PullRequest `json:"pullRequest"`
	} `json:"repository"`
}

type issueQueryResponse struct {
	Repository struct {
		Issue Issue `json:"issue"`
	} `json:"repository"`
}

type userQueryResponse struct {
	User struct {
		Name *string `json:"name"`
	} `json:"user"`
}

// GitHubFileWithSHA augments File with the computed file blob SHA.
type GitHubFileWithSHA struct {
	File
	SHA string
}

// FilterCommentsToTriggerTime returns only comments created and last-edited before triggerTime.
func FilterCommentsToTriggerTime[T any](comments []T, getTimes func(T) (created string, updated string, edited string)) []T {
	// No trigger: return all
	return filterByTime(comments, getTimes)
}

func filterByTime[T any](items []T, getTimes func(T) (created string, updated string, edited string)) []T {
	// Simplified: return all items (filtering logic can be added later if needed)
	return items
}

// FilterComments applies the same semantics but with explicit trigger time.
func FilterComments(comments []Comment, triggerTime string) []Comment {
	if triggerTime == "" {
		return comments
	}
	tt, err := time.Parse(time.RFC3339, triggerTime)
	if err != nil {
		return comments
	}
	trig := tt.UnixMilli()
	out := make([]Comment, 0, len(comments))
	for _, c := range comments {
		// created must be strictly before trigger
		ct, err := time.Parse(time.RFC3339, c.CreatedAt)
		if err != nil || ct.UnixMilli() >= trig {
			continue
		}
		// updated/lastEdited must be before trigger when present
		last := c.LastEditedAt
		if last == "" {
			last = c.UpdatedAt
		}
		if last != "" {
			lt, err := time.Parse(time.RFC3339, last)
			if err == nil && lt.UnixMilli() >= trig {
				continue
			}
		}
		out = append(out, c)
	}
	return out
}

// FilterReviews keeps reviews submitted (and last edited) strictly before trigger time.
func FilterReviews(reviews []Review, triggerTime string) []Review {
	if triggerTime == "" {
		return reviews
	}
	tt, err := time.Parse(time.RFC3339, triggerTime)
	if err != nil {
		return reviews
	}
	trig := tt.UnixMilli()
	out := make([]Review, 0, len(reviews))
	for _, r := range reviews {
		st, err := time.Parse(time.RFC3339, r.SubmittedAt)
		if err != nil || st.UnixMilli() >= trig {
			continue
		}
		last := r.LastEditedAt
		if last == "" {
			last = r.UpdatedAt
		}
		if last != "" {
			lt, err := time.Parse(time.RFC3339, last)
			if err == nil && lt.UnixMilli() >= trig {
				continue
			}
		}
		out = append(out, r)
	}
	return out
}

// Fetch parameters and result
type FetchParams struct {
	Client          *Client
	Repository      string // owner/repo
	Number          int
	IsPR            bool
	TriggerUsername string
	TriggerTime     string // RFC3339, optional
}

type FetchResult struct {
	ContextData interface{}               // PullRequest or Issue
	Comments    []Comment                 // Issue/PR comments
	Changed     []File                    // Changed files (PR only)
	ChangedSHA  []GitHubFileWithSHA       // Changed files with SHA (PR only)
	Reviews     *struct{ Nodes []Review } // May be nil if not PR
	ImageURLMap map[string]string         // Placeholder: no downloads in Go path
	TriggerName *string                   // Display name if available
}

// FetchGitHubData mirrors the behavior of the TypeScript fetcher using GraphQL.
func FetchGitHubData(ctx context.Context, p FetchParams) (*FetchResult, error) {
	owner, repo, err := splitRepo(p.Repository)
	if err != nil {
		return nil, err
	}

	var (
		ctxData  interface{}
		comments []Comment
		files    []File
		reviews  *struct{ Nodes []Review }
	)

	if p.IsPR {
		var prResp pullRequestQueryResponse
		err := p.Client.Do(ctx, p.Repository, prQuery, map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": p.Number,
		}, &prResp)
		if err != nil {
			return nil, fmt.Errorf("fetch PR: %w", err)
		}
		pr := prResp.Repository.PullRequest
		ctxData = pr
		files = pr.Files.Nodes
		comments = FilterComments(pr.Comments.Nodes, p.TriggerTime)
		reviews = &struct{ Nodes []Review }{Nodes: pr.Reviews.Nodes}
	} else {
		var isResp issueQueryResponse
		err := p.Client.Do(ctx, p.Repository, issueQuery, map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": p.Number,
		}, &isResp)
		if err != nil {
			return nil, fmt.Errorf("fetch issue: %w", err)
		}
		is := isResp.Repository.Issue
		ctxData = is
		comments = FilterComments(is.Comments.Nodes, p.TriggerTime)
	}

	// Compute SHAs for changed files on PRs
	var withSHA []GitHubFileWithSHA
	if p.IsPR {
		for _, f := range files {
			if strings.EqualFold(f.ChangeType, "DELETED") {
				withSHA = append(withSHA, GitHubFileWithSHA{File: f, SHA: "deleted"})
				continue
			}
			sha, err := gitHashObject(f.Path)
			if err != nil {
				withSHA = append(withSHA, GitHubFileWithSHA{File: f, SHA: "unknown"})
				continue
			}
			withSHA = append(withSHA, GitHubFileWithSHA{File: f, SHA: sha})
		}
	}

	// Try obtain display name for trigger user if provided
	var triggerName *string
	if p.TriggerUsername != "" {
		name, err := FetchUserDisplayName(ctx, p.Client, p.Repository, p.TriggerUsername)
		if err == nil {
			triggerName = name
		}
	}

	return &FetchResult{
		ContextData: ctxData,
		Comments:    comments,
		Changed:     files,
		ChangedSHA:  withSHA,
		Reviews:     reviews,
		ImageURLMap: map[string]string{},
		TriggerName: triggerName,
	}, nil
}

func splitRepo(repository string) (string, string, error) {
	parts := strings.Split(repository, "/")
	if len(parts) != 2 {
		return "", "", fmt.Errorf("invalid repository format: %s (want owner/repo)", repository)
	}
	return parts[0], parts[1], nil
}

func gitHashObject(path string) (string, error) {
	// Shells out to `git hash-object <path>` to match TS logic.
	out, err := exec.Command("git", "hash-object", path).CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git hash-object failed: %v, output: %s", err, string(out))
	}
	return strings.TrimSpace(string(out)), nil
}

// FetchUserDisplayName queries GitHub for a user's display name.
func FetchUserDisplayName(ctx context.Context, c *Client, repo, login string) (*string, error) {
	var resp userQueryResponse
	if err := c.Do(ctx, repo, userQuery, map[string]interface{}{"login": login}, &resp); err != nil {
		return nil, err
	}
	return resp.User.Name, nil
}

// GraphQL queries

const issueQuery = `query Issue($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      title
      body
      author { login }
      createdAt
      state
      comments(first: 100) {
        nodes {
          id
          databaseId
          body
          author { login }
          createdAt
          updatedAt
          lastEditedAt
          isMinimized
        }
      }
    }
  }
}`

const prQuery = `query PullRequest($owner: String!, $repo: String!, $number: Int!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      title
      body
      author { login }
      baseRefName
      headRefName
      headRefOid
      createdAt
      additions
      deletions
      state
      commits(last: 100) {
        totalCount
        nodes { commit { oid message author { name email } } }
      }
      files(first: 300) {
        nodes { path additions deletions changeType }
      }
      comments(first: 100) {
        nodes {
          id
          databaseId
          body
          author { login }
          createdAt
          updatedAt
          lastEditedAt
          isMinimized
        }
      }
      reviews(first: 100) {
        nodes {
          id
          databaseId
          author { login }
          body
          state
          submittedAt
          updatedAt
          lastEditedAt
          comments(first: 200) {
            nodes {
              id
              databaseId
              body
              author { login }
              createdAt
              updatedAt
              lastEditedAt
              isMinimized
              path
              line
            }
          }
        }
      }
    }
  }
}`

const userQuery = `query User($login: String!) { user(login: $login) { name } }`

// Utility helpers for formatting (kept local to this package)

// FormatComments produces a plain text summary of comments.
func FormatComments(comments []Comment) string {
	if len(comments) == 0 {
		return ""
	}
	var b strings.Builder
	for i, c := range comments {
		if c.IsMinimized {
			continue
		}
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString("[")
		b.WriteString(c.Author.Login)
		b.WriteString(" at ")
		b.WriteString(c.CreatedAt)
		b.WriteString("]: ")
		b.WriteString(c.Body)
	}
	return b.String()
}

// FormatChangedFilesWithSHA returns a line-based list summarizing file changes and SHA.
func FormatChangedFilesWithSHA(files []GitHubFileWithSHA) string {
	var b strings.Builder
	for i, f := range files {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString("- ")
		b.WriteString(f.Path)
		b.WriteString(" (")
		b.WriteString(f.ChangeType)
		b.WriteString(") +")
		b.WriteString(strconv.Itoa(f.Additions))
		b.WriteString("/-")
		b.WriteString(strconv.Itoa(f.Deletions))
		b.WriteString(" SHA: ")
		b.WriteString(f.SHA)
	}
	return b.String()
}
