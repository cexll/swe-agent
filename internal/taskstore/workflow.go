package taskstore

import (
	"sync"
	"time"
)

// WorkflowStage represents the current stage in the workflow
type WorkflowStage string

const (
	StageClarify  WorkflowStage = "clarify"
	StagePRD      WorkflowStage = "prd"
	StageCoding   WorkflowStage = "coding"
	StageReview   WorkflowStage = "review"
	StageDone     WorkflowStage = "done"
)

// Clarification represents a Q&A pair from the requirement clarification stage
type Clarification struct {
	Question  string    `json:"question"`
	Answer    string    `json:"answer"`
	Resolved  bool      `json:"resolved"`
	Timestamp time.Time `json:"timestamp"`
}

// WorkflowState tracks the state of a workflow through multiple stages
type WorkflowState struct {
	IssueNumber int `json:"issue_number"`
	Repo        string `json:"repo"`
	Stage       WorkflowStage `json:"stage"`
	
	// Clarification stage
	Clarifications []Clarification `json:"clarifications"`
	
	// PRD stage
	PRD string `json:"prd"`
	
	// Coding stage
	BranchName   string   `json:"branch_name"`
	FilesChanged []string `json:"files_changed"`
	
	// Review stage
	FixAttempts    int `json:"fix_attempts"`
	MaxFixAttempts int `json:"max_fix_attempts"`
	
	// Cost tracking
	TotalCost float64 `json:"total_cost"`
	
	// Time tracking
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// WorkflowStore manages workflow states
type WorkflowStore struct {
	mu        sync.RWMutex
	workflows map[string]*WorkflowState // key: "{repo}#{issue_number}"
}

// NewWorkflowStore creates a new workflow store
func NewWorkflowStore() *WorkflowStore {
	return &WorkflowStore{
		workflows: make(map[string]*WorkflowState),
	}
}

// NewWorkflowState creates a new workflow state with defaults
func NewWorkflowState(repo string, issueNumber int) *WorkflowState {
	now := time.Now()
	return &WorkflowState{
		IssueNumber:    issueNumber,
		Repo:           repo,
		Stage:          "", // Empty stage means not started
		Clarifications: []Clarification{},
		BranchName:     "",
		FilesChanged:   []string{},
		FixAttempts:    0,
		MaxFixAttempts: 3, // Default max fix attempts
		TotalCost:      0.0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
}

// getWorkflowKey returns the map key for a workflow
func (s *WorkflowStore) getWorkflowKey(repo string, issueNumber int) string {
	return repo + "#" + string(rune(issueNumber))
}

// GetWorkflow retrieves a workflow state
func (s *WorkflowStore) GetWorkflow(repo string, issueNumber int) *WorkflowState {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	if workflow, exists := s.workflows[key]; exists {
		return workflow
	}
	
	// Return a new workflow state if not found
	return NewWorkflowState(repo, issueNumber)
}

// UpdateWorkflow updates a workflow state
func (s *WorkflowStore) UpdateWorkflow(workflow *WorkflowState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := s.getWorkflowKey(workflow.Repo, workflow.IssueNumber)
	workflow.UpdatedAt = time.Now()
	s.workflows[key] = workflow
}

// AddClarification adds a new clarification to the workflow
func (s *WorkflowStore) AddClarification(repo string, issueNumber int, question, answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	workflow := s.workflows[key]
	if workflow == nil {
		workflow = NewWorkflowState(repo, issueNumber)
		s.workflows[key] = workflow
	}
	
	clarification := Clarification{
		Question:  question,
		Answer:    answer,
		Resolved:  answer != "",
		Timestamp: time.Now(),
	}
	
	workflow.Clarifications = append(workflow.Clarifications, clarification)
	workflow.UpdatedAt = time.Now()
}

// UpdatePRD updates the PRD for a workflow
func (s *WorkflowStore) UpdatePRD(repo string, issueNumber int, prd string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	workflow := s.workflows[key]
	if workflow == nil {
		workflow = NewWorkflowState(repo, issueNumber)
		s.workflows[key] = workflow
	}
	
	workflow.PRD = prd
	workflow.Stage = StagePRD
	workflow.UpdatedAt = time.Now()
}

// UpdateCodingInfo updates the coding stage information
func (s *WorkflowStore) UpdateCodingInfo(repo string, issueNumber int, branchName string, filesChanged []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	workflow := s.workflows[key]
	if workflow == nil {
		workflow = NewWorkflowState(repo, issueNumber)
		s.workflows[key] = workflow
	}
	
	workflow.BranchName = branchName
	workflow.FilesChanged = filesChanged
	workflow.Stage = StageCoding
	workflow.UpdatedAt = time.Now()
}

// IncrementFixAttempts increments the fix attempts counter
func (s *WorkflowStore) IncrementFixAttempts(repo string, issueNumber int) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	workflow := s.workflows[key]
	if workflow == nil {
		workflow = NewWorkflowState(repo, issueNumber)
		s.workflows[key] = workflow
	}
	
	workflow.FixAttempts++
	workflow.Stage = StageReview
	workflow.UpdatedAt = time.Now()
	
	return workflow.FixAttempts <= workflow.MaxFixAttempts
}

// AddCost adds to the total cost for a workflow
func (s *WorkflowStore) AddCost(repo string, issueNumber int, cost float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	workflow := s.workflows[key]
	if workflow == nil {
		workflow = NewWorkflowState(repo, issueNumber)
		s.workflows[key] = workflow
	}
	
	workflow.TotalCost += cost
	workflow.UpdatedAt = time.Now()
}

// SetStage updates the workflow stage
func (s *WorkflowStore) SetStage(repo string, issueNumber int, stage WorkflowStage) {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	workflow := s.workflows[key]
	if workflow == nil {
		workflow = NewWorkflowState(repo, issueNumber)
		s.workflows[key] = workflow
	}
	
	workflow.Stage = stage
	workflow.UpdatedAt = time.Now()
}

// CanAttemptFix checks if a fix attempt is allowed
func (s *WorkflowStore) CanAttemptFix(repo string, issueNumber int) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	key := s.getWorkflowKey(repo, issueNumber)
	workflow := s.workflows[key]
	if workflow == nil {
		return true // No workflow yet, allow fix
	}
	
	return workflow.FixAttempts < workflow.MaxFixAttempts
}