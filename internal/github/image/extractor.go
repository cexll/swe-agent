package image

import (
	"regexp"
	"strings"
)

// ExtractImageURLs 从文本中提取所有图片 URL
// 支持的格式：
// - Markdown: ![alt](url)
// - HTML: <img src="url">
// - 直接 URL: https://...jpg/png/gif
func ExtractImageURLs(text string) []string {
	var urls []string
	seen := make(map[string]bool)

	// 1. Markdown 格式: ![alt](url)
	markdownRe := regexp.MustCompile(`!\[.*?\]\((https?://[^\)]+)\)`)
	for _, match := range markdownRe.FindAllStringSubmatch(text, -1) {
		url := match[1]
		if !seen[url] && isImageURL(url) {
			urls = append(urls, url)
			seen[url] = true
		}
	}

	// 2. HTML img 标签: <img src="url">
	htmlRe := regexp.MustCompile(`<img[^>]+src=["']([^"']+)["']`)
	for _, match := range htmlRe.FindAllStringSubmatch(text, -1) {
		url := match[1]
		if !seen[url] && isImageURL(url) {
			urls = append(urls, url)
			seen[url] = true
		}
	}

	// 3. 直接图片 URL
	directRe := regexp.MustCompile(`https?://[^\s<>"']+\.(?:jpg|jpeg|png|gif|webp|svg)(?:\?[^\s<>"']*)?`)
	for _, match := range directRe.FindAllString(text, -1) {
		if !seen[match] {
			urls = append(urls, match)
			seen[match] = true
		}
	}

	return urls
}

// isImageURL 检查 URL 是否看起来像图片
func isImageURL(url string) bool {
	imageExtensions := []string{".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg"}
	urlLower := strings.ToLower(url)

	for _, ext := range imageExtensions {
		if strings.Contains(urlLower, ext) {
			return true
		}
	}

	// GitHub 用户内容 URL
	if strings.Contains(urlLower, "user-images.githubusercontent.com") ||
		strings.Contains(urlLower, "github.com/user-attachments") {
		return true
	}

	return false
}
