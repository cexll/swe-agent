package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	gh "github.com/google/go-github/v66/github"
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

// CommitFilesOptions 提交选项
type CommitFilesOptions struct {
	Owner       string
	Repo        string
	Branch      string
	Message     string
	Files       map[string]string // path -> content (text)
	Sign        bool              // 是否签名（GraphQL SIGN_WITH_GITHUB）
	AuthorName  string
	AuthorEmail string
}

// CommitFiles 通过 API 提交多个文件（支持签名）
// 当 Sign=true 时，使用 GraphQL createCommitOnBranch 并启用 SIGN_WITH_GITHUB。
// 否则，使用 REST v3（trees/commits/refs）路径。
func CommitFiles(ctx context.Context, client *gh.Client, opts CommitFilesOptions) (string, error) {
	if client == nil {
		return "", fmt.Errorf("nil github client")
	}
	if opts.Owner == "" || opts.Repo == "" || opts.Branch == "" {
		return "", fmt.Errorf("missing owner/repo/branch")
	}
	if len(opts.Files) == 0 {
		return "", fmt.Errorf("no files to commit")
	}
	if opts.Sign {
		return commitFilesGraphQL(ctx, client, opts)
	}
	return commitFilesREST(ctx, client, opts)
}

func commitFilesREST(ctx context.Context, client *gh.Client, opts CommitFilesOptions) (string, error) {
	baseURL := client.BaseURL.String()

	// 1) Get or create branch ref
	headSHA, err := apiDoWithClient(ctx, client, "GET", fmt.Sprintf("%srepos/%s/%s/git/refs/heads/%s", baseURL, opts.Owner, opts.Repo, opts.Branch), nil)
	var sha string
	if err == nil {
		var r gitRef
		if uerr := json.Unmarshal(headSHA, &r); uerr != nil {
			return "", fmt.Errorf("parse ref: %w", uerr)
		}
		sha = r.Object.SHA
	} else {
		// Try default branch
		rb, rerr := apiDoWithClient(ctx, client, "GET", fmt.Sprintf("%srepos/%s/%s", baseURL, opts.Owner, opts.Repo), nil)
		if rerr != nil {
			return "", fmt.Errorf("get repo: %w", rerr)
		}
		var info repoInfo
		if uerr := json.Unmarshal(rb, &info); uerr != nil {
			return "", fmt.Errorf("parse repo: %w", uerr)
		}
		bb, berr := apiDoWithClient(ctx, client, "GET", fmt.Sprintf("%srepos/%s/%s/git/refs/heads/%s", baseURL, opts.Owner, opts.Repo, info.DefaultBranch), nil)
		if berr != nil {
			return "", fmt.Errorf("get default ref: %w", berr)
		}
		var br gitRef
		if uerr := json.Unmarshal(bb, &br); uerr != nil {
			return "", fmt.Errorf("parse default ref: %w", uerr)
		}
		sha = br.Object.SHA
		// create branch
		_, cerr := apiDoWithClient(ctx, client, "POST", fmt.Sprintf("%srepos/%s/%s/git/refs", baseURL, opts.Owner, opts.Repo), map[string]string{
			"ref": fmt.Sprintf("refs/heads/%s", opts.Branch),
			"sha": sha,
		})
		if cerr != nil {
			return "", fmt.Errorf("create branch: %w", cerr)
		}
	}

	// 2) Base commit -> base tree
	cb, cerr := apiDoWithClient(ctx, client, "GET", fmt.Sprintf("%srepos/%s/%s/git/commits/%s", baseURL, opts.Owner, opts.Repo, sha), nil)
	if cerr != nil {
		return "", fmt.Errorf("get base commit: %w", cerr)
	}
	var base gitCommit
	if uerr := json.Unmarshal(cb, &base); uerr != nil {
		return "", fmt.Errorf("parse base commit: %w", uerr)
	}

	// 3) Create tree
	var entries []map[string]any
	for p, content := range opts.Files {
		entries = append(entries, map[string]any{
			"path":    p,
			"mode":    "100644",
			"type":    "blob",
			"content": content,
		})
	}
	tb, terr := apiDoWithClient(ctx, client, "POST", fmt.Sprintf("%srepos/%s/%s/git/trees", baseURL, opts.Owner, opts.Repo), map[string]any{
		"base_tree": base.Tree.SHA,
		"tree":      entries,
	})
	if terr != nil {
		return "", fmt.Errorf("create tree: %w", terr)
	}
	var tree gitTree
	if uerr := json.Unmarshal(tb, &tree); uerr != nil {
		return "", fmt.Errorf("parse tree: %w", uerr)
	}

	// 4) Create commit (optionally include author)
	body := map[string]any{
		"message": opts.Message,
		"tree":    tree.SHA,
		"parents": []string{sha},
	}
	if opts.AuthorName != "" || opts.AuthorEmail != "" {
		body["author"] = map[string]string{
			"name":  opts.AuthorName,
			"email": opts.AuthorEmail,
		}
	}
	nb, nerr := apiDoWithClient(ctx, client, "POST", fmt.Sprintf("%srepos/%s/%s/git/commits", baseURL, opts.Owner, opts.Repo), body)
	if nerr != nil {
		return "", fmt.Errorf("create commit: %w", nerr)
	}
	var newC gitNewCommit
	if uerr := json.Unmarshal(nb, &newC); uerr != nil {
		return "", fmt.Errorf("parse new commit: %w", uerr)
	}

	// 5) Update ref
	if _, uerr := apiDoWithClient(ctx, client, "PATCH", fmt.Sprintf("%srepos/%s/%s/git/refs/heads/%s", baseURL, opts.Owner, opts.Repo, opts.Branch), map[string]any{
		"sha":   newC.SHA,
		"force": false,
	}); uerr != nil {
		return "", fmt.Errorf("update ref: %w", uerr)
	}

	return newC.SHA, nil
}

func commitFilesGraphQL(ctx context.Context, client *gh.Client, opts CommitFilesOptions) (string, error) {
	baseURL := client.BaseURL
	if baseURL == nil {
		return "", fmt.Errorf("nil base url")
	}
	// Fetch head oid for expectedHeadOid
	rb, rerr := apiDoWithClient(ctx, client, "GET", fmt.Sprintf("%srepos/%s/%s/git/refs/heads/%s", baseURL.String(), opts.Owner, opts.Repo, opts.Branch), nil)
	var headOID string
	if rerr == nil {
		var r gitRef
		if uerr := json.Unmarshal(rb, &r); uerr == nil {
			headOID = r.Object.SHA
		}
	} else {
		// If not exists, create from default branch
		infoB, ierr := apiDoWithClient(ctx, client, "GET", fmt.Sprintf("%srepos/%s/%s", baseURL.String(), opts.Owner, opts.Repo), nil)
		if ierr != nil {
			return "", fmt.Errorf("get repo: %w", ierr)
		}
		var info repoInfo
		if uerr := json.Unmarshal(infoB, &info); uerr != nil {
			return "", fmt.Errorf("parse repo: %w", uerr)
		}
		defB, derr := apiDoWithClient(ctx, client, "GET", fmt.Sprintf("%srepos/%s/%s/git/refs/heads/%s", baseURL.String(), opts.Owner, opts.Repo, info.DefaultBranch), nil)
		if derr != nil {
			return "", fmt.Errorf("get default branch: %w", derr)
		}
		var dr gitRef
		if uerr := json.Unmarshal(defB, &dr); uerr != nil {
			return "", fmt.Errorf("parse default ref: %w", uerr)
		}
		headOID = dr.Object.SHA
		// create branch ref
		if _, cerr := apiDoWithClient(ctx, client, "POST", fmt.Sprintf("%srepos/%s/%s/git/refs", baseURL.String(), opts.Owner, opts.Repo), map[string]string{
			"ref": fmt.Sprintf("refs/heads/%s", opts.Branch),
			"sha": headOID,
		}); cerr != nil {
			return "", fmt.Errorf("create branch: %w", cerr)
		}
	}

	// Build GraphQL mutation
	gqlURL := fmt.Sprintf("%sgraphql", baseURL.Scheme+"://"+baseURL.Host+"/")
	type addition struct {
		Path     string `json:"path"`
		Contents string `json:"contents"`
	}
	var adds []addition
	for p, c := range opts.Files {
		adds = append(adds, addition{Path: p, Contents: c})
	}
	payload := map[string]any{
		"query": `mutation($input: CreateCommitOnBranchInput!){
  createCommitOnBranch(input: $input){
    commit { oid messageHeadline committedDate }
  }
}`,
		"variables": map[string]any{
			"input": map[string]any{
				"branch": map[string]any{
					"repositoryNameWithOwner": fmt.Sprintf("%s/%s", opts.Owner, opts.Repo),
					"branchName":              fmt.Sprintf("refs/heads/%s", opts.Branch),
				},
				"message": map[string]any{
					"headline": opts.Message,
				},
				"fileChanges": map[string]any{
					"additions": adds,
				},
				"expectedHeadOid": headOID,
				"signing":         "SIGN_WITH_GITHUB",
			},
		},
	}

	respB, err := apiDoWithClient(ctx, client, "POST", gqlURL, payload)
	if err != nil {
		return "", fmt.Errorf("graphql commit: %w", err)
	}
	var resp struct {
		Data struct {
			CreateCommitOnBranch struct {
				Commit struct {
					OID string `json:"oid"`
				} `json:"commit"`
			} `json:"createCommitOnBranch"`
		} `json:"data"`
		Errors any `json:"errors"`
	}
	if uerr := json.Unmarshal(respB, &resp); uerr != nil {
		return "", fmt.Errorf("parse graphql response: %w", uerr)
	}
	if resp.Data.CreateCommitOnBranch.Commit.OID == "" {
		return "", fmt.Errorf("graphql commit returned empty oid")
	}
	return resp.Data.CreateCommitOnBranch.Commit.OID, nil
}

func apiDoWithClient(ctx context.Context, client *gh.Client, method, url string, v any) ([]byte, error) {
	var body io.Reader
	if v != nil {
		b, err := json.Marshal(v)
		if err != nil {
			return nil, fmt.Errorf("marshal: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("new request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	if v != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	// Use the underlying http.Client from go-github (already has auth transport)
	httpClient := client.Client()
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read resp: %w", err)
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
