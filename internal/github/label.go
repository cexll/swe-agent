package github

// AddLabel adds a label to an issue or PR
func AddLabel(repo string, number int, label string, token string) error {
	return defaultGHClient.AddLabel(repo, number, label, token)
}
