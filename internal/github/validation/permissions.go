package validation

import (
	"context"
	"fmt"

	"github.com/google/go-github/v66/github"
)

// CheckWritePermission 检查用户是否有写权限
// 返回 true 表示有 write 或 admin 权限
func CheckWritePermission(ctx context.Context, client *github.Client, owner, repo, user string) (bool, error) {
	perm, _, err := client.Repositories.GetPermissionLevel(ctx, owner, repo, user)
	if err != nil {
		return false, fmt.Errorf("failed to get permission level: %w", err)
	}

	permission := perm.GetPermission()

	// write 或 admin 权限都允许
	return permission == "write" || permission == "admin", nil
}

// CheckAdminPermission 检查用户是否有管理员权限
func CheckAdminPermission(ctx context.Context, client *github.Client, owner, repo, user string) (bool, error) {
	perm, _, err := client.Repositories.GetPermissionLevel(ctx, owner, repo, user)
	if err != nil {
		return false, fmt.Errorf("failed to get permission level: %w", err)
	}

	return perm.GetPermission() == "admin", nil
}

// EnsureWritePermission 确保用户有写权限，否则返回错误
func EnsureWritePermission(ctx context.Context, client *github.Client, owner, repo, user string) error {
	hasWrite, err := CheckWritePermission(ctx, client, owner, repo, user)
	if err != nil {
		return err
	}

	if !hasWrite {
		return fmt.Errorf("user %s lacks write permission on %s/%s", user, owner, repo)
	}

	return nil
}
