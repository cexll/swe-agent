package prompt

import (
	"fmt"
)

// ClarifyPrompt builds a prompt for requirement clarification (Stage 0)
func (m Manager) ClarifyPrompt(issueTitle, issueBody, content string, repoContext string) string {
	return fmt.Sprintf(`You are a requirements analysis expert. Please analyze the following GitHub issue and generate clarification questions.

【Issue Title】%s
【Issue Body】%s
【Additional Context】%s

【Repository Context】
%s

Please propose 5-10 clarification questions to ensure the requirements are clear. Focus on:

1. **Functional Boundaries**: What features are included/excluded?
2. **Technical Constraints**: Version compatibility, performance requirements?
3. **Acceptance Criteria**: How do we determine the feature is complete?
4. **Dependencies**: Are there other features or systems this depends on?

Output format (Markdown Checklist):
- [ ] **Question 1**: [Specific question about scope/features]
- [ ] **Question 2**: [Specific question about technical constraints]
- [ ] **Question 3**: [Specific question about acceptance criteria]
- [ ] **Question 4**: [Specific question about dependencies]
- [ ] **Question 5**: [Specific question about edge cases]

If the repository structure or existing code provides relevant context, mention specific files or components that need clarification.`, issueTitle, issueBody, content, repoContext)
}

// PRDPrompt builds a prompt for PRD generation (Stage 1)
func (m Manager) PRDPrompt(issueTitle, issueBody, content string, repoContext string, clarifications string) string {
	return fmt.Sprintf(`You are a product manager and technical architect. Based on the GitHub issue and any clarification answers, generate a comprehensive Product Requirements Document (PRD).

【Issue Title】%s
【Issue Body】%s
【Additional Context】%s

【Repository Context】
%s

【Clarification History】
%s

Please create a structured PRD with the following sections:

## Background
[Context about the problem or opportunity]

## Objectives
- **Primary Goal**: [Main objective]
- **Success Metrics**: [How we measure success]

## Non-Objectives
- What this feature will NOT address
- Explicitly out of scope items

## Technical Approach
- Architecture overview
- Key components to be modified/created
- Integration points

## Acceptance Criteria
- [ ] **Criterion 1**: [Specific, measurable requirement]
- [ ] **Criterion 2**: [Specific, measurable requirement]
- [ ] **Criterion 3**: [Specific, measurable requirement]

## Implementation Plan
### Files to be modified/created:
- `path/to/file1.ext` - [Brief description of changes]
- `path/to/file2.ext` - [Brief description of changes]

### Development phases:
1. **Phase 1**: [Description]
2. **Phase 2**: [Description] (if applicable)

## Risk Assessment
- **Technical Risks**: [Potential implementation challenges]
- **Mitigation Strategies**: [How to address the risks]

## Dependencies
- **External Dependencies**: [APIs, services, etc.]
- **Internal Dependencies**: [Other features or teams]

Keep the PRD concise but comprehensive. Focus on clear requirements and technical feasibility.`, issueTitle, issueBody, content, repoContext, clarifications)
}

// CodeReviewPrompt builds a prompt for code review (Stage 3)
func (m Manager) CodeReviewPrompt(content string, prdSummary string, fileChanges string) string {
	return fmt.Sprintf(`You are a senior software engineer performing a code review. Please analyze the following code changes.

【Additional Context】
%s

【PRD Summary】
%s

【File Changes】
%s

Please provide a comprehensive code review focusing on:

### ✅ Code Quality Checklist
- [ ] **Code Style**: Follows project conventions and best practices
- [ ] **Error Handling**: Proper error handling and edge cases covered
- [ ] **Performance**: No obvious performance issues or resource leaks
- [ ] **Security**: No security vulnerabilities or exposed secrets
- [ ] **Testing**: Appropriate test coverage for new functionality
- [ ] **Documentation**: Code is well-documented and self-explanatory

### 🔍 Specific Issues Found
If you find any issues, please provide:

**File**: `path/to/file.ext:line_number`
**Issue**: [Description of the problem]
**Severity**: [High/Medium/Low]
**Suggestion**: [Recommended fix or improvement]

### 📝 General Feedback
- Overall assessment of the implementation
- Alignment with PRD requirements
- Any architectural concerns
- Suggestions for improvement

Please format your response as a structured review that can be posted as a GitHub comment.`, content, prdSummary, fileChanges)
}

// WorkflowEnhancedCodePrompt builds an enhanced prompt for code implementation (Stage 2)
func (m Manager) WorkflowEnhancedCodePrompt(issueTitle, issueBody, content string, repoContext string, prdSummary string, clarifications string) string {
	return fmt.Sprintf(`You are implementing a GitHub issue following a structured workflow. Please analyze the requirements and implement the solution.

## Issue Context
【Issue Title】%s
【Issue Body】%s
【Additional Instructions】%s

## Repository Context
%s

## PRD Summary (if available)
%s

## Clarification History (if available)
%s

## Implementation Guidelines

### 1. Analysis Phase
First, analyze the existing codebase:
- Identify relevant files and components
- Understand existing patterns and conventions
- Look for similar implementations for reference

### 2. Design Phase
- Plan the minimal changes needed
- Consider edge cases and error handling
- Ensure backward compatibility

### 3. Implementation Phase
- Follow the project's coding style
- Make focused, minimal changes
- Add appropriate comments where the logic is complex
- Include error handling for all external dependencies

### 4. Testing Phase
- Add or update unit tests if the project has a test suite
- Consider integration scenarios
- Test edge cases identified in the analysis

## Output Format

Please provide your implementation using this format:

<file path="relative/path/to/file.ext">
<content>
[Complete file content with your changes]
</content>
</file>

<file path="another/file.ext" (if needed)>
<content>
[Complete file content with your changes]
</content>
</file>

<summary>
[Brief description of what was implemented and why]
</summary>

## Important Notes
- **Minimal Changes**: Only modify what's necessary to address the requirements
- **Backward Compatibility**: Don't break existing functionality
- **Error Handling**: Include proper error handling and logging
- **Code Style**: Match the existing code style and patterns
- **Documentation**: Add comments for complex logic or business rules`, issueTitle, issueBody, content, repoContext, prdSummary, clarifications)
}