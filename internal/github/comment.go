package github

// Global gh client instance (can be replaced for testing)
var defaultGHClient GHClient = NewRealGHClient()

// SetGHClient allows replacing the global gh client (useful for testing)
func SetGHClient(client GHClient) {
	defaultGHClient = client
}

// CreateComment creates a comment on a GitHub issue or PR using GitHub App authentication with retry logic
func CreateComment(repo string, number int, body string, token string) error {
	_, err := CreateCommentWithID(repo, number, body, token)
	return err
}

// CreateCommentWithID creates a comment and returns its ID
func CreateCommentWithID(repo string, number int, body string, token string) (int, error) {
	return defaultGHClient.CreateComment(repo, number, body, token)
}

// UpdateComment updates an existing comment
func UpdateComment(repo string, commentID int, body string, token string) error {
	return defaultGHClient.UpdateComment(repo, commentID, body, token)
}

// GetCommentBody retrieves the current body of a comment
func GetCommentBody(repo string, commentID int, token string) (string, error) {
	return defaultGHClient.GetCommentBody(repo, commentID, token)
}
