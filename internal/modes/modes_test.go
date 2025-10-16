package modes_test

import (
	"testing"

	ghctx "github.com/cexll/swe/internal/github"
	"github.com/cexll/swe/internal/modes"
	_ "github.com/cexll/swe/internal/modes/command"
)

func TestRegister(t *testing.T) {
	// Command 模式应该已经自动注册
	mode, err := modes.Get("command")
	if err != nil {
		t.Fatalf("Failed to get command mode: %v", err)
	}

	if mode.Name() != "command" {
		t.Errorf("Expected mode name 'command', got %s", mode.Name())
	}
}

func TestDetectMode(t *testing.T) {
	tests := []struct {
		name        string
		commentBody string
		shouldMatch bool
	}{
		{name: "contains /code", commentBody: "Please fix this bug /code", shouldMatch: true},
		{name: "only /code", commentBody: "/code", shouldMatch: true},
		{name: "case insensitive", commentBody: "/CODE", shouldMatch: true},
		{name: "no command", commentBody: "Just a regular comment", shouldMatch: false},
		{name: "contains @claude (not supported)", commentBody: "@claude help me", shouldMatch: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := &ghctx.Context{
				TriggerComment: &ghctx.Comment{Body: tt.commentBody},
			}

			mode := modes.DetectMode(ctx)

			if tt.shouldMatch && mode == nil {
				t.Error("Expected mode to be detected, but got nil")
			}
			if !tt.shouldMatch && mode != nil {
				t.Errorf("Expected no mode, but got %s", mode.Name())
			}
			if tt.shouldMatch && mode != nil && mode.Name() != "command" {
				t.Errorf("Expected command mode, got %s", mode.Name())
			}
		})
	}
}

func TestGetCommandMode(t *testing.T) {
	mode := modes.GetCommandMode()
	if mode == nil {
		t.Fatal("GetCommandMode returned nil")
	}
	if mode.Name() != "command" {
		t.Errorf("Expected command mode, got %s", mode.Name())
	}
}
