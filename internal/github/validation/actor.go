package validation

import (
	"strings"

	"github.com/google/go-github/v66/github"
)

// IsBot 检测用户是否为 Bot
// Bot 的特征：
// 1. Type 为 "Bot"
// 2. Login 以 [bot] 结尾
func IsBot(user *github.User) bool {
	if user == nil {
		return false
	}

	// 检查 Type
	if user.GetType() == "Bot" {
		return true
	}

	// 检查 Login 是否以 [bot] 结尾
	login := user.GetLogin()
	return strings.HasSuffix(login, "[bot]")
}

// IsBotLogin 仅根据 login 字符串判断是否为 Bot
func IsBotLogin(login string) bool {
	return strings.HasSuffix(login, "[bot]")
}

// ShouldIgnoreActor 判断是否应该忽略此 Actor
// 用于防止 Bot 之间的死循环
func ShouldIgnoreActor(user *github.User, appBotLogin string) bool {
	if user == nil {
		return true
	}

	login := user.GetLogin()

	// 忽略自己
	if login == appBotLogin {
		return true
	}

	// 忽略所有 Bot（可选，根据需求调整）
	if IsBot(user) {
		return true
	}

	return false
}
