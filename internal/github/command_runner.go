package github

import "os/exec"

// CommandRunner is an interface for executing system commands
// This abstraction allows us to mock command execution in tests
type CommandRunner interface {
	// Run executes a command and returns the combined output and error
	Run(name string, args ...string) ([]byte, error)

	// RunInDir executes a command in a specific directory
	RunInDir(dir, name string, args ...string) ([]byte, error)
}

// RealCommandRunner is the production implementation using os/exec
type RealCommandRunner struct{}

// Run executes a command using os/exec
func (r *RealCommandRunner) Run(name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	return cmd.CombinedOutput()
}

// RunInDir executes a command in a specific directory
func (r *RealCommandRunner) RunInDir(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}

// MockCommandRunner is a test implementation that returns predefined responses
type MockCommandRunner struct {
	// RunFunc is called when Run is invoked
	RunFunc func(name string, args ...string) ([]byte, error)

	// RunInDirFunc is called when RunInDir is invoked
	RunInDirFunc func(dir, name string, args ...string) ([]byte, error)

	// Calls tracks all command invocations
	Calls []MockCall
}

// MockCall represents a single command invocation
type MockCall struct {
	Name string
	Args []string
	Dir  string
}

// Run executes the mock function
func (m *MockCommandRunner) Run(name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, MockCall{Name: name, Args: args})

	if m.RunFunc != nil {
		return m.RunFunc(name, args...)
	}

	return []byte(""), nil
}

// RunInDir executes the mock function with directory context
func (m *MockCommandRunner) RunInDir(dir, name string, args ...string) ([]byte, error) {
	m.Calls = append(m.Calls, MockCall{Name: name, Args: args, Dir: dir})

	if m.RunInDirFunc != nil {
		return m.RunInDirFunc(dir, name, args...)
	}

	return []byte(""), nil
}

// NewMockCommandRunner creates a new mock with default behavior
func NewMockCommandRunner() *MockCommandRunner {
	return &MockCommandRunner{
		Calls: make([]MockCall, 0),
	}
}
