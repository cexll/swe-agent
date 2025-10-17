package git

import (
	"fmt"
	"os/exec"
)

// ConfigureGit 配置 Git 用户信息
// 用于在容器或 CI 环境中自动配置 Git
func ConfigureGit(botName, botEmail string) error {
	// 设置 user.name
	cmd := exec.Command("git", "config", "--global", "user.name", botName)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user.name: %w", err)
	}

	// 设置 user.email
	cmd = exec.Command("git", "config", "--global", "user.email", botEmail)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to set git user.email: %w", err)
	}

	return nil
}

// ConfigureGitForApp 为 GitHub App 配置 Git
// botID: GitHub App 的 Bot ID
// appName: GitHub App 名称（默认 "swe-agent"）
func ConfigureGitForApp(botID int, appName string) error {
	if appName == "" {
		appName = "swe-agent"
	}

	botName := fmt.Sprintf("%s[bot]", appName)
	botEmail := fmt.Sprintf("%d+%s[bot]@users.noreply.github.com", botID, appName)

	return ConfigureGit(botName, botEmail)
}

// GetGitConfig 获取当前 Git 配置
func GetGitConfig(key string) (string, error) {
	cmd := exec.Command("git", "config", "--global", "--get", key)
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
