package validation

import (
	"testing"

	"github.com/google/go-github/v66/github"
)

func TestIsBot(t *testing.T) {
	tests := []struct {
		name string
		user *github.User
		want bool
	}{
		{
			name: "nil user",
			user: nil,
			want: false,
		},
		{
			name: "bot type",
			user: &github.User{
				Type: github.String("Bot"),
			},
			want: true,
		},
		{
			name: "bot login suffix",
			user: &github.User{
				Type:  github.String("User"),
				Login: github.String("dependabot[bot]"),
			},
			want: true,
		},
		{
			name: "regular user",
			user: &github.User{
				Type:  github.String("User"),
				Login: github.String("octocat"),
			},
			want: false,
		},
		{
			name: "github-actions bot",
			user: &github.User{
				Type:  github.String("Bot"),
				Login: github.String("github-actions[bot]"),
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsBot(tt.user)
			if got != tt.want {
				t.Errorf("IsBot() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBotLogin(t *testing.T) {
	tests := []struct {
		login string
		want  bool
	}{
		{"dependabot[bot]", true},
		{"github-actions[bot]", true},
		{"swe-agent[bot]", true},
		{"octocat", false},
		{"user-bot", false}, // 不以 [bot] 结尾
	}

	for _, tt := range tests {
		t.Run(tt.login, func(t *testing.T) {
			got := IsBotLogin(tt.login)
			if got != tt.want {
				t.Errorf("IsBotLogin(%q) = %v, want %v", tt.login, got, tt.want)
			}
		})
	}
}

func TestShouldIgnoreActor(t *testing.T) {
	appBotLogin := "swe-agent[bot]"

	tests := []struct {
		name   string
		user   *github.User
		ignore bool
	}{
		{
			name:   "nil user",
			user:   nil,
			ignore: true,
		},
		{
			name: "self bot",
			user: &github.User{
				Login: github.String(appBotLogin),
			},
			ignore: true,
		},
		{
			name: "other bot",
			user: &github.User{
				Type:  github.String("Bot"),
				Login: github.String("dependabot[bot]"),
			},
			ignore: true,
		},
		{
			name: "regular user",
			user: &github.User{
				Type:  github.String("User"),
				Login: github.String("octocat"),
			},
			ignore: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ShouldIgnoreActor(tt.user, appBotLogin)
			if got != tt.ignore {
				t.Errorf("ShouldIgnoreActor() = %v, want %v", got, tt.ignore)
			}
		})
	}
}
