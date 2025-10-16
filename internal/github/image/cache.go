package image

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// CleanupCache 清理过期缓存
// maxAge: 文件最大保留时间
func (d *Downloader) CleanupCache(maxAge time.Duration) error {
	entries, err := os.ReadDir(d.cacheDir)
	if err != nil {
		return err
	}

	now := time.Now()
	cleaned := 0

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		// 检查文件年龄
		if now.Sub(info.ModTime()) > maxAge {
			path := filepath.Join(d.cacheDir, entry.Name())
			if err := os.Remove(path); err != nil {
				continue
			}
			cleaned++
		}
	}

	if cleaned > 0 {
		fmt.Printf("Cleaned up %d cached images\n", cleaned)
	}

	return nil
}

// GetCacheSize 获取缓存总大小（字节）
func (d *Downloader) GetCacheSize() (int64, error) {
	entries, err := os.ReadDir(d.cacheDir)
	if err != nil {
		return 0, err
	}

	var totalSize int64
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		info, err := entry.Info()
		if err != nil {
			continue
		}

		totalSize += info.Size()
	}

	return totalSize, nil
}

// ClearCache 清空所有缓存
func (d *Downloader) ClearCache() error {
	return os.RemoveAll(d.cacheDir)
}
