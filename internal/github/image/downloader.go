package image

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Downloader 图片下载器
type Downloader struct {
	cacheDir   string
	httpClient *http.Client
}

// NewDownloader 创建图片下载器
// cacheDir: 缓存目录，如果为空则使用临时目录
func NewDownloader(cacheDir string) (*Downloader, error) {
	if cacheDir == "" {
		cacheDir = filepath.Join(os.TempDir(), "swe-agent-images")
	}

	// 确保缓存目录存在
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create cache dir: %w", err)
	}

	return &Downloader{
		cacheDir: cacheDir,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// DownloadImages 下载多个图片
// 返回 URL -> 本地路径的映射
func (d *Downloader) DownloadImages(ctx context.Context, urls []string) (map[string]string, error) {
	result := make(map[string]string)

	for _, url := range urls {
		localPath, err := d.Download(ctx, url)
		if err != nil {
			// 下载失败时记录但继续
			fmt.Printf("Warning: Failed to download image %s: %v\n", url, err)
			continue
		}
		result[url] = localPath
	}

	return result, nil
}

// Download 下载单个图片
func (d *Downloader) Download(ctx context.Context, url string) (string, error) {
	// 1. 生成缓存文件名
	filename := d.generateFilename(url)
	localPath := filepath.Join(d.cacheDir, filename)

	// 2. 检查是否已缓存
	if _, err := os.Stat(localPath); err == nil {
		return localPath, nil
	}

	// 3. 下载图片
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	// 4. 保存到本地
	file, err := os.Create(localPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	if _, err := io.Copy(file, resp.Body); err != nil {
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return localPath, nil
}

// generateFilename 生成缓存文件名
// 使用 URL 的 SHA256 哈希 + 扩展名
func (d *Downloader) generateFilename(url string) string {
	// 计算 URL 的 SHA256
	hash := sha256.Sum256([]byte(url))
	hashStr := fmt.Sprintf("%x", hash[:8]) // 使用前 8 字节

	// 提取文件扩展名
	ext := extractExtension(url)

	return hashStr + ext
}

// extractExtension 提取文件扩展名
func extractExtension(url string) string {
	// 移除查询参数
	if idx := strings.Index(url, "?"); idx != -1 {
		url = url[:idx]
	}

	// 提取扩展名
	ext := filepath.Ext(url)
	if ext == "" {
		ext = ".png" // 默认扩展名
	}

	return ext
}
