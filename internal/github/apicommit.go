package github

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const githubAPIBase = "https://api.github.com"

type gitRef struct {
	Object struct {
		SHA string `json:"sha"`
	} `json:"object"`
}

type gitCommit struct {
	Tree struct {
		SHA string `json:"sha"`
	} `json:"tree"`
}

type gitTree struct {
	SHA string `json:"sha"`
}

type gitNewCommit struct {
	SHA     string `json:"sha"`
	Message string `json:"message"`
	Author  struct {
		Name string `json:"name"`
		Date string `json:"date"`
	} `json:"author"`
}

type repoInfo struct {
	DefaultBranch string `json:"default_branch"`
}

type APIFile struct {
	Path    string
	Content []byte
	Mode    string
	Binary  bool
}

func CommitFilesAPI(owner, repo, branch, baseBranch, message, token string, files []APIFile) (string, error) {
	sha, err := getOrCreateBranchRef(owner, repo, branch, baseBranch, token)
	if err != nil {
		return "", fmt.Errorf("failed to get/create branch ref: %w", err)
	}

	baseCommit, err := getCommit(owner, repo, sha, token)
	if err != nil {
		return "", fmt.Errorf("failed to get base commit: %w", err)
	}

	treeSHA, err := createTree(owner, repo, baseCommit.Tree.SHA, token, files)
	if err != nil {
		return "", fmt.Errorf("failed to create tree: %w", err)
	}

	newCommit, err := createCommit(owner, repo, message, treeSHA, sha, token)
	if err != nil {
		return "", fmt.Errorf("failed to create commit: %w", err)
	}

	if err := updateRefWithRetry(owner, repo, branch, newCommit.SHA, token); err != nil {
		return "", fmt.Errorf("failed to update ref: %w", err)
	}

	return newCommit.SHA, nil
}

func getOrCreateBranchRef(owner, repo, branch, baseBranch, token string) (string, error) {
	if sha, err := getRef(owner, repo, "heads/"+branch, token); err == nil {
		return sha, nil
	}

	baseSHA, err := getRef(owner, repo, "heads/"+baseBranch, token)
	if err != nil {
		def, derr := apiGET(fmt.Sprintf("%s/repos/%s/%s", githubAPIBase, owner, repo), token)
		if derr != nil {
			return "", fmt.Errorf("failed to get repository info: %w", derr)
		}

		var repoData repoInfo
		if err := json.Unmarshal(def, &repoData); err != nil {
			return "", fmt.Errorf("failed to parse repository info: %w", err)
		}

		baseSHA, err = getRef(owner, repo, "heads/"+repoData.DefaultBranch, token)
		if err != nil {
			return "", fmt.Errorf("failed to get default branch ref: %w", err)
		}
	}

	body := map[string]string{
		"ref": "refs/heads/" + branch,
		"sha": baseSHA,
	}

	_, err = apiPOST(fmt.Sprintf("%s/repos/%s/%s/git/refs", githubAPIBase, owner, repo), token, body)
	if err != nil {
		return "", fmt.Errorf("failed to create branch: %w", err)
	}

	return baseSHA, nil
}

func getRef(owner, repo, ref, token string) (string, error) {
	b, err := apiGET(fmt.Sprintf("%s/repos/%s/%s/git/refs/%s", githubAPIBase, owner, repo, ref), token)
	if err != nil {
		return "", err
	}

	var r gitRef
	if err := json.Unmarshal(b, &r); err != nil {
		return "", fmt.Errorf("failed to parse ref: %w", err)
	}

	return r.Object.SHA, nil
}

func getCommit(owner, repo, sha, token string) (*gitCommit, error) {
	b, err := apiGET(fmt.Sprintf("%s/repos/%s/%s/git/commits/%s", githubAPIBase, owner, repo, sha), token)
	if err != nil {
		return nil, err
	}

	var c gitCommit
	if err := json.Unmarshal(b, &c); err != nil {
		return nil, fmt.Errorf("failed to parse commit: %w", err)
	}

	return &c, nil
}

func createTree(owner, repo, baseTree, token string, files []APIFile) (string, error) {
	var entries []map[string]any

	for _, f := range files {
		mode := f.Mode
		if mode == "" {
			mode = "100644"
		}

		if f.Binary {
			blobBody := map[string]string{
				"content":  base64.StdEncoding.EncodeToString(f.Content),
				"encoding": "base64",
			}

			bb, err := apiPOST(fmt.Sprintf("%s/repos/%s/%s/git/blobs", githubAPIBase, owner, repo), token, blobBody)
			if err != nil {
				return "", fmt.Errorf("failed to create blob for %s: %w", f.Path, err)
			}

			var bj struct {
				SHA string `json:"sha"`
			}
			if err := json.Unmarshal(bb, &bj); err != nil {
				return "", fmt.Errorf("failed to parse blob response: %w", err)
			}

			entries = append(entries, map[string]any{
				"path": f.Path,
				"mode": mode,
				"type": "blob",
				"sha":  bj.SHA,
			})
		} else {
			entries = append(entries, map[string]any{
				"path":    f.Path,
				"mode":    mode,
				"type":    "blob",
				"content": string(f.Content),
			})
		}
	}

	body := map[string]any{
		"base_tree": baseTree,
		"tree":      entries,
	}

	b, err := apiPOST(fmt.Sprintf("%s/repos/%s/%s/git/trees", githubAPIBase, owner, repo), token, body)
	if err != nil {
		return "", err
	}

	var t gitTree
	if err := json.Unmarshal(b, &t); err != nil {
		return "", fmt.Errorf("failed to parse tree: %w", err)
	}

	return t.SHA, nil
}

func createCommit(owner, repo, msg, treeSHA, parent, token string) (*gitNewCommit, error) {
	body := map[string]any{
		"message": msg,
		"tree":    treeSHA,
		"parents": []string{parent},
	}

	b, err := apiPOST(fmt.Sprintf("%s/repos/%s/%s/git/commits", githubAPIBase, owner, repo), token, body)
	if err != nil {
		return nil, err
	}

	var nc gitNewCommit
	if err := json.Unmarshal(b, &nc); err != nil {
		return nil, fmt.Errorf("failed to parse commit: %w", err)
	}

	return &nc, nil
}

func updateRefWithRetry(owner, repo, branch, sha, token string) error {
	backoff := time.Second
	maxAttempts := 3

	for attempt := 0; attempt < maxAttempts; attempt++ {
		body := map[string]any{
			"sha":   sha,
			"force": false,
		}

		_, err := apiPATCH(fmt.Sprintf("%s/repos/%s/%s/git/refs/heads/%s", githubAPIBase, owner, repo, branch), token, body)
		if err == nil {
			return nil
		}

		errMsg := strings.ToLower(err.Error())
		if !strings.Contains(errMsg, "403") || attempt == maxAttempts-1 {
			if strings.Contains(errMsg, "403") {
				return fmt.Errorf("permission denied: unable to push commits to branch '%s': %w", branch, err)
			}
			return err
		}

		time.Sleep(backoff)
		backoff *= 2
	}

	return fmt.Errorf("update ref failed after %d attempts", maxAttempts)
}

func apiGET(url, token string) ([]byte, error) {
	return apiDo("GET", url, token, nil)
}

func apiPOST(url, token string, v any) ([]byte, error) {
	return apiDo("POST", url, token, v)
}

func apiPATCH(url, token string, v any) ([]byte, error) {
	return apiDo("PATCH", url, token, v)
}

func apiDo(method, url, token string, v any) ([]byte, error) {
	var body io.Reader
	if v != nil {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		body = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if v != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("%s %s failed with status %d: %s", method, url, resp.StatusCode, string(b))
	}

	return b, nil
}

func CollectChangedFilesForAPICommit(workdir string) ([]APIFile, error) {
	cmd := exec.Command("git", "status", "--porcelain", "--untracked-files=all")
	cmd.Dir = workdir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git status failed: %w", err)
	}

	var files []APIFile
	lines := strings.Split(strings.TrimSpace(string(output)), "\n")

	for _, line := range lines {
		if line == "" {
			continue
		}

		if len(line) < 4 {
			continue
		}

		filePath := strings.TrimSpace(line[3:])
		if filePath == "" || strings.HasSuffix(filePath, "/") {
			continue
		}

		fullPath := filepath.Join(workdir, filePath)
		content, err := os.ReadFile(fullPath)
		if err != nil {
			continue
		}

		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			continue
		}

		mode := "100644"
		if fileInfo.Mode()&0111 != 0 {
			mode = "100755"
		}

		isBinary := isBinaryFile(filePath)

		files = append(files, APIFile{
			Path:    filePath,
			Content: content,
			Mode:    mode,
			Binary:  isBinary,
		})
	}

	return files, nil
}

func isBinaryFile(path string) bool {
	binaryExts := []string{
		".png", ".jpg", ".jpeg", ".gif", ".webp", ".ico",
		".pdf", ".zip", ".tar", ".gz", ".exe", ".bin",
		".woff", ".woff2", ".ttf", ".eot",
	}

	lowerPath := strings.ToLower(path)
	for _, ext := range binaryExts {
		if strings.HasSuffix(lowerPath, ext) {
			return true
		}
	}

	return false
}
