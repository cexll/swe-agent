package github

import (
	"fmt"
	"strings"
	"testing"
)

func TestRealCommandRunner_Run(t *testing.T) {
	runner := &RealCommandRunner{}

	// Test successful command
	output, err := runner.Run("echo", "hello")
	if err != nil {
		t.Errorf("Run() unexpected error: %v", err)
	}
	if !strings.Contains(string(output), "hello") {
		t.Errorf("Run() output = %q, want to contain 'hello'", string(output))
	}
}

func TestRealCommandRunner_RunInDir(t *testing.T) {
	runner := &RealCommandRunner{}

	// Test command execution in a directory
	output, err := runner.RunInDir("/tmp", "pwd")
	if err != nil {
		t.Errorf("RunInDir() unexpected error: %v", err)
	}
	if len(output) == 0 {
		t.Error("RunInDir() returned empty output")
	}
}

func TestMockCommandRunner_Run(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func(*MockCommandRunner)
		command    string
		args       []string
		wantOutput string
		wantErr    bool
	}{
		{
			name: "default behavior (no func set)",
			setupMock: func(m *MockCommandRunner) {
				// No setup, use default behavior
			},
			command:    "test",
			args:       []string{"arg1", "arg2"},
			wantOutput: "",
			wantErr:    false,
		},
		{
			name: "custom function returns output",
			setupMock: func(m *MockCommandRunner) {
				m.RunFunc = func(name string, args ...string) ([]byte, error) {
					return []byte("custom output"), nil
				}
			},
			command:    "test",
			args:       []string{"arg1"},
			wantOutput: "custom output",
			wantErr:    false,
		},
		{
			name: "custom function returns error",
			setupMock: func(m *MockCommandRunner) {
				m.RunFunc = func(name string, args ...string) ([]byte, error) {
					return nil, fmt.Errorf("command failed")
				}
			},
			command:    "test",
			args:       []string{},
			wantOutput: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommandRunner()
			tt.setupMock(mock)

			output, err := mock.Run(tt.command, tt.args...)

			if (err != nil) != tt.wantErr {
				t.Errorf("Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if string(output) != tt.wantOutput {
				t.Errorf("Run() output = %q, want %q", string(output), tt.wantOutput)
			}

			// Verify call was recorded
			if len(mock.Calls) != 1 {
				t.Errorf("Expected 1 call, got %d", len(mock.Calls))
			}

			if len(mock.Calls) > 0 {
				call := mock.Calls[0]
				if call.Name != tt.command {
					t.Errorf("Call name = %s, want %s", call.Name, tt.command)
				}
				if len(call.Args) != len(tt.args) {
					t.Errorf("Call args length = %d, want %d", len(call.Args), len(tt.args))
				}
			}
		})
	}
}

func TestMockCommandRunner_RunInDir(t *testing.T) {
	tests := []struct {
		name       string
		setupMock  func(*MockCommandRunner)
		dir        string
		command    string
		args       []string
		wantOutput string
		wantErr    bool
	}{
		{
			name: "default behavior",
			setupMock: func(m *MockCommandRunner) {
				// No setup
			},
			dir:        "/tmp",
			command:    "test",
			args:       []string{"arg1"},
			wantOutput: "",
			wantErr:    false,
		},
		{
			name: "custom function returns output",
			setupMock: func(m *MockCommandRunner) {
				m.RunInDirFunc = func(dir, name string, args ...string) ([]byte, error) {
					return []byte(fmt.Sprintf("executed in %s", dir)), nil
				}
			},
			dir:        "/custom/dir",
			command:    "test",
			args:       []string{},
			wantOutput: "executed in /custom/dir",
			wantErr:    false,
		},
		{
			name: "custom function returns error",
			setupMock: func(m *MockCommandRunner) {
				m.RunInDirFunc = func(dir, name string, args ...string) ([]byte, error) {
					return nil, fmt.Errorf("directory error")
				}
			},
			dir:        "/tmp",
			command:    "test",
			args:       []string{},
			wantOutput: "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := NewMockCommandRunner()
			tt.setupMock(mock)

			output, err := mock.RunInDir(tt.dir, tt.command, tt.args...)

			if (err != nil) != tt.wantErr {
				t.Errorf("RunInDir() error = %v, wantErr %v", err, tt.wantErr)
			}

			if string(output) != tt.wantOutput {
				t.Errorf("RunInDir() output = %q, want %q", string(output), tt.wantOutput)
			}

			// Verify call was recorded with directory
			if len(mock.Calls) != 1 {
				t.Errorf("Expected 1 call, got %d", len(mock.Calls))
			}

			if len(mock.Calls) > 0 {
				call := mock.Calls[0]
				if call.Dir != tt.dir {
					t.Errorf("Call dir = %s, want %s", call.Dir, tt.dir)
				}
				if call.Name != tt.command {
					t.Errorf("Call name = %s, want %s", call.Name, tt.command)
				}
			}
		})
	}
}

func TestNewMockCommandRunner(t *testing.T) {
	mock := NewMockCommandRunner()

	if mock == nil {
		t.Fatal("NewMockCommandRunner() returned nil")
	}

	if mock.Calls == nil {
		t.Error("NewMockCommandRunner() Calls should not be nil")
	}

	if len(mock.Calls) != 0 {
		t.Errorf("NewMockCommandRunner() Calls length = %d, want 0", len(mock.Calls))
	}
}

func TestMockCommandRunner_CallTracking(t *testing.T) {
	mock := NewMockCommandRunner()

	// Make multiple calls
	mock.Run("cmd1", "arg1")
	mock.RunInDir("/dir1", "cmd2", "arg2", "arg3")
	mock.Run("cmd3")

	if len(mock.Calls) != 3 {
		t.Fatalf("Expected 3 calls, got %d", len(mock.Calls))
	}

	// Verify first call
	if mock.Calls[0].Name != "cmd1" {
		t.Errorf("Call[0] name = %s, want cmd1", mock.Calls[0].Name)
	}
	if len(mock.Calls[0].Args) != 1 {
		t.Errorf("Call[0] args length = %d, want 1", len(mock.Calls[0].Args))
	}

	// Verify second call
	if mock.Calls[1].Name != "cmd2" {
		t.Errorf("Call[1] name = %s, want cmd2", mock.Calls[1].Name)
	}
	if mock.Calls[1].Dir != "/dir1" {
		t.Errorf("Call[1] dir = %s, want /dir1", mock.Calls[1].Dir)
	}
	if len(mock.Calls[1].Args) != 2 {
		t.Errorf("Call[1] args length = %d, want 2", len(mock.Calls[1].Args))
	}

	// Verify third call
	if mock.Calls[2].Name != "cmd3" {
		t.Errorf("Call[2] name = %s, want cmd3", mock.Calls[2].Name)
	}
	if len(mock.Calls[2].Args) != 0 {
		t.Errorf("Call[2] args length = %d, want 0", len(mock.Calls[2].Args))
	}
}

func TestRealCommandRunner_RunError(t *testing.T) {
	runner := &RealCommandRunner{}

	// Test command that should fail
	_, err := runner.Run("nonexistent-command-xyz")
	if err == nil {
		t.Error("Run() should return error for nonexistent command")
	}
}

func TestRealCommandRunner_RunInDirError(t *testing.T) {
	runner := &RealCommandRunner{}

	// Test command that should fail
	_, err := runner.RunInDir("/tmp", "nonexistent-command-xyz")
	if err == nil {
		t.Error("RunInDir() should return error for nonexistent command")
	}
}
