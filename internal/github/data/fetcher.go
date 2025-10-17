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

type PageInfo struct {
	HasNextPage bool   `json:"hasNextPage"`
	EndCursor   string `json:"endCursor"`
}

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

type FilesConnection struct {
	PageInfo PageInfo `json:"pageInfo"`
	Nodes    []File   `json:"nodes"`
}

type CommentsConnection struct {
	PageInfo PageInfo  `json:"pageInfo"`
	Nodes    []Comment `json:"nodes"`
}

type ReviewCommentsConnection struct {
	PageInfo PageInfo        `json:"pageInfo"`
	Nodes    []ReviewComment `json:"nodes"`
}

type ReviewsConnection struct {
	PageInfo PageInfo `json:"pageInfo"`
	Nodes    []Review `json:"nodes"`
}

type Review struct {
	ID           string                   `json:"id"`
	DatabaseID   int                      `json:"databaseId"`
	Author       Author                   `json:"author"`
	Body         string                   `json:"body"`
	State        string                   `json:"state"`
	SubmittedAt  string                   `json:"submittedAt"`
	UpdatedAt    string                   `json:"updatedAt,omitempty"`
	LastEditedAt string                   `json:"lastEditedAt,omitempty"`
	Comments     ReviewCommentsConnection `json:"comments"`
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
	Files    FilesConnection    `json:"files"`
	Comments CommentsConnection `json:"comments"`
	Reviews  ReviewsConnection  `json:"reviews"`
}

type Issue struct {
	Title     string             `json:"title"`
	Body      string             `json:"body"`
	Author    Author             `json:"author"`
	CreatedAt string             `json:"createdAt"`
	State     string             `json:"state"`
	Comments  CommentsConnection `json:"comments"`
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

		// Fetch files with pagination
		files = pr.Files.Nodes
		if pr.Files.PageInfo.HasNextPage {
			moreFiles, err := fetchAllRemainingFiles(ctx, p.Client, owner, repo, p.Number, pr.Files.PageInfo.EndCursor)
			if err != nil {
				return nil, fmt.Errorf("fetch remaining files: %w", err)
			}
			files = append(files, moreFiles...)
		}

		// Fetch comments with pagination
		comments = pr.Comments.Nodes
		if pr.Comments.PageInfo.HasNextPage {
			moreComments, err := fetchAllRemainingComments(ctx, p.Client, owner, repo, p.Number, pr.Comments.PageInfo.EndCursor, true)
			if err != nil {
				return nil, fmt.Errorf("fetch remaining PR comments: %w", err)
			}
			comments = append(comments, moreComments...)
		}
		comments = FilterComments(comments, p.TriggerTime)

		// Fetch reviews with pagination
		reviewNodes := pr.Reviews.Nodes
		if pr.Reviews.PageInfo.HasNextPage {
			moreReviews, err := fetchAllRemainingReviews(ctx, p.Client, owner, repo, p.Number, pr.Reviews.PageInfo.EndCursor)
			if err != nil {
				return nil, fmt.Errorf("fetch remaining reviews: %w", err)
			}
			reviewNodes = append(reviewNodes, moreReviews...)
		}

		// Fetch review comments with pagination for each review
		for i := range reviewNodes {
			review := &reviewNodes[i]
			if review.Comments.PageInfo.HasNextPage {
				moreReviewComments, err := fetchAllReviewComments(ctx, p.Client, p.Repository, review.ID, review.Comments.PageInfo.EndCursor)
				if err != nil {
					return nil, fmt.Errorf("fetch remaining review comments for review %s: %w", review.ID, err)
				}
				review.Comments.Nodes = append(review.Comments.Nodes, moreReviewComments...)
			}
		}

		reviews = &struct{ Nodes []Review }{Nodes: reviewNodes}
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

		// Fetch issue comments with pagination
		comments = is.Comments.Nodes
		if is.Comments.PageInfo.HasNextPage {
			moreComments, err := fetchAllRemainingComments(ctx, p.Client, owner, repo, p.Number, is.Comments.PageInfo.EndCursor, false)
			if err != nil {
				return nil, fmt.Errorf("fetch remaining issue comments: %w", err)
			}
			comments = append(comments, moreComments...)
		}
		comments = FilterComments(comments, p.TriggerTime)
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

// Pagination helper functions

const maxPaginationIterations = 50

// fetchAllRemainingFiles fetches all remaining files from a PR using cursor-based pagination.
func fetchAllRemainingFiles(ctx context.Context, c *Client, owner, repo string, number int, cursor string) ([]File, error) {
	var allFiles []File
	currentCursor := cursor
	iterations := 0

	for currentCursor != "" && iterations < maxPaginationIterations {
		iterations++

		type filesResponse struct {
			Repository struct {
				PullRequest struct {
					Files FilesConnection `json:"files"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}

		var resp filesResponse
		err := c.Do(ctx, owner+"/"+repo, fetchMoreFilesQuery, map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"cursor": currentCursor,
		}, &resp)

		if err != nil {
			return nil, fmt.Errorf("fetch more files: %w", err)
		}

		allFiles = append(allFiles, resp.Repository.PullRequest.Files.Nodes...)

		if !resp.Repository.PullRequest.Files.PageInfo.HasNextPage {
			break
		}
		currentCursor = resp.Repository.PullRequest.Files.PageInfo.EndCursor
	}

	return allFiles, nil
}

// fetchAllRemainingComments fetches all remaining comments using cursor-based pagination.
func fetchAllRemainingComments(ctx context.Context, c *Client, owner, repo string, number int, cursor string, isPR bool) ([]Comment, error) {
	var allComments []Comment
	currentCursor := cursor
	iterations := 0

	query := fetchMoreIssueCommentsQuery
	if isPR {
		query = fetchMorePRCommentsQuery
	}

	for currentCursor != "" && iterations < maxPaginationIterations {
		iterations++

		type commentsResponse struct {
			Repository struct {
				PullRequest *struct {
					Comments CommentsConnection `json:"comments"`
				} `json:"pullRequest,omitempty"`
				Issue *struct {
					Comments CommentsConnection `json:"comments"`
				} `json:"issue,omitempty"`
			} `json:"repository"`
		}

		var resp commentsResponse
		err := c.Do(ctx, owner+"/"+repo, query, map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"cursor": currentCursor,
		}, &resp)

		if err != nil {
			return nil, fmt.Errorf("fetch more comments: %w", err)
		}

		var conn CommentsConnection
		if isPR && resp.Repository.PullRequest != nil {
			conn = resp.Repository.PullRequest.Comments
		} else if !isPR && resp.Repository.Issue != nil {
			conn = resp.Repository.Issue.Comments
		}

		allComments = append(allComments, conn.Nodes...)

		if !conn.PageInfo.HasNextPage {
			break
		}
		currentCursor = conn.PageInfo.EndCursor
	}

	return allComments, nil
}

// fetchAllRemainingReviews fetches all remaining reviews using cursor-based pagination.
func fetchAllRemainingReviews(ctx context.Context, c *Client, owner, repo string, number int, cursor string) ([]Review, error) {
	var allReviews []Review
	currentCursor := cursor
	iterations := 0

	for currentCursor != "" && iterations < maxPaginationIterations {
		iterations++

		type reviewsResponse struct {
			Repository struct {
				PullRequest struct {
					Reviews ReviewsConnection `json:"reviews"`
				} `json:"pullRequest"`
			} `json:"repository"`
		}

		var resp reviewsResponse
		err := c.Do(ctx, owner+"/"+repo, fetchMoreReviewsQuery, map[string]interface{}{
			"owner":  owner,
			"repo":   repo,
			"number": number,
			"cursor": currentCursor,
		}, &resp)

		if err != nil {
			return nil, fmt.Errorf("fetch more reviews: %w", err)
		}

		allReviews = append(allReviews, resp.Repository.PullRequest.Reviews.Nodes...)

		if !resp.Repository.PullRequest.Reviews.PageInfo.HasNextPage {
			break
		}
		currentCursor = resp.Repository.PullRequest.Reviews.PageInfo.EndCursor
	}

	return allReviews, nil
}

// fetchAllReviewComments fetches all comments for a specific review using cursor-based pagination.
func fetchAllReviewComments(ctx context.Context, c *Client, repo, reviewID, cursor string) ([]ReviewComment, error) {
	var allComments []ReviewComment
	currentCursor := cursor
	iterations := 0

	for currentCursor != "" && iterations < maxPaginationIterations {
		iterations++

		type reviewCommentsResponse struct {
			Node struct {
				Comments ReviewCommentsConnection `json:"comments"`
			} `json:"node"`
		}

		var resp reviewCommentsResponse
		err := c.Do(ctx, repo, fetchMoreReviewCommentsQuery, map[string]interface{}{
			"reviewId": reviewID,
			"cursor":   currentCursor,
		}, &resp)

		if err != nil {
			return nil, fmt.Errorf("fetch more review comments: %w", err)
		}

		allComments = append(allComments, resp.Node.Comments.Nodes...)

		if !resp.Node.Comments.PageInfo.HasNextPage {
			break
		}
		currentCursor = resp.Node.Comments.PageInfo.EndCursor
	}

	return allComments, nil
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
        pageInfo { hasNextPage endCursor }
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
      files(first: 100) {
        pageInfo { hasNextPage endCursor }
        nodes { path additions deletions changeType }
      }
      comments(first: 100) {
        pageInfo { hasNextPage endCursor }
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
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          databaseId
          author { login }
          body
          state
          submittedAt
          updatedAt
          lastEditedAt
          comments(first: 100) {
            pageInfo { hasNextPage endCursor }
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

const fetchMoreFilesQuery = `query FetchMoreFiles($owner: String!, $repo: String!, $number: Int!, $cursor: String!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      files(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes { path additions deletions changeType }
      }
    }
  }
}`

const fetchMorePRCommentsQuery = `query FetchMorePRComments($owner: String!, $repo: String!, $number: Int!, $cursor: String!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      comments(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
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

const fetchMoreIssueCommentsQuery = `query FetchMoreIssueComments($owner: String!, $repo: String!, $number: Int!, $cursor: String!) {
  repository(owner: $owner, name: $repo) {
    issue(number: $number) {
      comments(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
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

const fetchMoreReviewsQuery = `query FetchMoreReviews($owner: String!, $repo: String!, $number: Int!, $cursor: String!) {
  repository(owner: $owner, name: $repo) {
    pullRequest(number: $number) {
      reviews(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
        nodes {
          id
          databaseId
          author { login }
          body
          state
          submittedAt
          updatedAt
          lastEditedAt
          comments(first: 100) {
            pageInfo { hasNextPage endCursor }
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

const fetchMoreReviewCommentsQuery = `query FetchMoreReviewComments($owner: String!, $repo: String!, $reviewId: ID!, $cursor: String!) {
  node(id: $reviewId) {
    ... on PullRequestReview {
      comments(first: 100, after: $cursor) {
        pageInfo { hasNextPage endCursor }
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
}`

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
