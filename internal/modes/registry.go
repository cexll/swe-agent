package modes

import (
	"fmt"

	"github.com/cexll/swe/internal/github"
)

var (
	// 全局模式注册表
	registeredModes = make(map[string]Mode)
)

// Register 注册模式
func Register(mode Mode) {
	registeredModes[mode.Name()] = mode
}

// Get 获取指定模式
func Get(name string) (Mode, error) {
	mode, ok := registeredModes[name]
	if !ok {
		return nil, fmt.Errorf("mode not found: %s", name)
	}
	return mode, nil
}

// DetectMode 自动检测应该使用的模式
func DetectMode(ctx *github.Context) Mode {
	for _, mode := range registeredModes {
		if mode.ShouldTrigger(ctx) {
			return mode
		}
	}
	return nil
}

// GetCommandMode 获取 Command 模式（便捷方法）
func GetCommandMode() Mode {
	mode, _ := Get("command")
	return mode
}

// init 初始化，注册默认模式
func init() {
	// Command 模式在 command 包中自动注册
}
