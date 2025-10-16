package image

import (
	"strings"
	"testing"
)

func TestExtractImageURLs(t *testing.T) {
	tests := []struct {
		name string
		text string
		want int
	}{
		{
			name: "markdown image",
			text: "Here is an image: ![alt](https://example.com/image.png)",
			want: 1,
		},
		{
			name: "html image",
			text: `<img src="https://example.com/photo.jpg" alt="test">`,
			want: 1,
		},
		{
			name: "direct url",
			text: "Check this: https://example.com/screenshot.png",
			want: 1,
		},
		{
			name: "github user-attachments",
			text: "![image](https://github.com/user-attachments/assets/abc123.png)",
			want: 1,
		},
		{
			name: "multiple images",
			text: "![img1](https://example.com/1.png) ![img2](https://example.com/2.jpg)",
			want: 2,
		},
		{
			name: "no images",
			text: "Just plain text with no images",
			want: 0,
		},
		{
			name: "duplicate urls",
			text: "![img](https://example.com/test.png) and again ![img](https://example.com/test.png)",
			want: 1, // 去重
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			urls := ExtractImageURLs(tt.text)
			if len(urls) != tt.want {
				t.Errorf("ExtractImageURLs() got %d URLs, want %d", len(urls), tt.want)
				t.Logf("URLs: %v", urls)
			}
		})
	}
}

func TestIsImageURL(t *testing.T) {
	tests := []struct {
		url  string
		want bool
	}{
		{"https://example.com/image.png", true},
		{"https://example.com/photo.jpg", true},
		{"https://example.com/pic.jpeg", true},
		{"https://example.com/anim.gif", true},
		{"https://example.com/icon.svg", true},
		{"https://user-images.githubusercontent.com/123/abc.png", true},
		{"https://github.com/user-attachments/assets/test.png", true},
		{"https://example.com/page.html", false},
		{"https://example.com/doc.pdf", false},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := isImageURL(tt.url)
			if got != tt.want {
				t.Errorf("isImageURL(%q) = %v, want %v", tt.url, got, tt.want)
			}
		})
	}
}

func TestGenerateFilename(t *testing.T) {
	d := &Downloader{}

	tests := []struct {
		url     string
		wantExt string
	}{
		{"https://example.com/image.png", ".png"},
		{"https://example.com/photo.jpg", ".jpg"},
		{"https://example.com/pic.gif?size=large", ".gif"},
		{"https://example.com/noext", ".png"}, // 默认扩展名
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			filename := d.generateFilename(tt.url)

			if !strings.HasSuffix(filename, tt.wantExt) {
				t.Errorf("generateFilename(%q) = %q, want suffix %q", tt.url, filename, tt.wantExt)
			}

			// 文件名应该包含哈希（16个字符 + 扩展名）
			if len(filename) < 16 {
				t.Errorf("generateFilename(%q) = %q, too short", tt.url, filename)
			}
		})
	}
}

func TestExtractExtension(t *testing.T) {
	tests := []struct {
		url  string
		want string
	}{
		{"https://example.com/image.png", ".png"},
		{"https://example.com/photo.jpg", ".jpg"},
		{"https://example.com/pic.gif?size=large", ".gif"},
		{"https://example.com/noext", ".png"},
	}

	for _, tt := range tests {
		t.Run(tt.url, func(t *testing.T) {
			got := extractExtension(tt.url)
			if got != tt.want {
				t.Errorf("extractExtension(%q) = %q, want %q", tt.url, got, tt.want)
			}
		})
	}
}
