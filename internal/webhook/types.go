package webhook

// GitHub webhook event types

type IssueCommentEvent struct {
	Action     string     `json:"action"`
	Issue      Issue      `json:"issue"`
	Comment    Comment    `json:"comment"`
	Repository Repository `json:"repository"`
	Sender     User       `json:"sender"`
}

type PullRequestReviewCommentEvent struct {
	Action      string        `json:"action"`
	Comment     ReviewComment `json:"comment"`
	PullRequest PullRequest   `json:"pull_request"`
	Repository  Repository    `json:"repository"`
	Sender      User          `json:"sender"`
}

type Issue struct {
	Number      int    `json:"number"`
	Title       string `json:"title"`
	Body        string `json:"body"`
	State       string `json:"state"`
	PullRequest *struct {
		URL string `json:"url"`
	} `json:"pull_request,omitempty"`
}

type Comment struct {
	ID   int64  `json:"id"`
	Body string `json:"body"`
	User User   `json:"user"`
}

type ReviewComment struct {
	ID       int64  `json:"id"`
	Body     string `json:"body"`
	User     User   `json:"user"`
	Path     string `json:"path"`
	DiffHunk string `json:"diff_hunk"`
}

type Repository struct {
	FullName      string `json:"full_name"`
	DefaultBranch string `json:"default_branch"`
	Owner         User   `json:"owner"`
	Name          string `json:"name"`
}

type PullRequest struct {
	Number int    `json:"number"`
	Title  string `json:"title"`
	Body   string `json:"body"`
	Base   struct {
		Ref string `json:"ref"`
	} `json:"base"`
}

type User struct {
	Login string `json:"login"`
	Type  string `json:"type"`
}
