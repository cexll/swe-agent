package github

import (
	"fmt"
	"log"
	"strings"

	"github.com/cexll/swe/internal/provider/claude"
)

// PRCategory represents the category of a PR
type PRCategory string

const (
	CategoryTests    PRCategory = "tests"
	CategoryDocs     PRCategory = "docs"
	CategoryInternal PRCategory = "internal"
	CategoryCore     PRCategory = "core"
	CategoryCmd      PRCategory = "cmd"
)

// SubPR represents a sub-PR in a split plan
type SubPR struct {
	Index       int                 // Index in the plan
	Name        string              // Human-readable name
	Description string              // Detailed description
	Files       []claude.FileChange // Files included in this sub-PR
	Category    PRCategory          // Category of this sub-PR
	DependsOn   []int               // Indices of sub-PRs this depends on
	Priority    int                 // Creation priority (lower = earlier)
}

// SplitPlan represents a plan to split changes into multiple PRs
type SplitPlan struct {
	SubPRs        []SubPR // List of sub-PRs
	CreationOrder []int   // Order to create PRs (indices into SubPRs)
	TotalFiles    int     // Total number of files
	TotalLines    int     // Total lines changed (approximate)
}

// PRSplitter analyzes file changes and generates split plans
type PRSplitter struct {
	maxFilesPerPR int // Maximum files per PR
	maxLinesPerPR int // Maximum lines per PR (approximate)
}

// NewPRSplitter creates a new PR splitter with default thresholds
func NewPRSplitter(maxFiles, maxLines int) *PRSplitter {
	if maxFiles <= 0 {
		maxFiles = 8 // Default: 8 files per PR
	}
	if maxLines <= 0 {
		maxLines = 300 // Default: 300 lines per PR
	}

	return &PRSplitter{
		maxFilesPerPR: maxFiles,
		maxLinesPerPR: maxLines,
	}
}

// Analyze analyzes file changes and generates a split plan
// If changes are small enough, returns a single-PR plan
// If changes are large, automatically splits into multiple PRs
func (s *PRSplitter) Analyze(files []claude.FileChange, prompt string) *SplitPlan {
	log.Printf("[Splitter] Analyzing %d files for splitting", len(files))

	// Step 1: Group files by category
	groups := s.groupByCategory(files)
	log.Printf("[Splitter] Grouped into %d categories", len(groups))

	// Step 2: Calculate total stats
	totalFiles := len(files)
	totalLines := s.estimateTotalLines(files)
	log.Printf("[Splitter] Total: %d files, ~%d lines", totalFiles, totalLines)

	// Step 3: Decide if splitting is needed
	needsSplit := s.shouldSplit(totalFiles, totalLines, groups)

	if !needsSplit {
		log.Printf("[Splitter] Changes are small, creating single PR")
		return s.createSinglePRPlan(files, totalFiles, totalLines)
	}

	log.Printf("[Splitter] Changes are large, splitting into multiple PRs")

	// Step 4: Generate sub-PRs from groups
	subPRs := s.generateSubPRs(groups)
	log.Printf("[Splitter] Generated %d sub-PRs", len(subPRs))

	// Step 5: Determine creation order based on dependencies
	order := s.determineCreationOrder(subPRs)
	log.Printf("[Splitter] Creation order: %v", order)

	return &SplitPlan{
		SubPRs:        subPRs,
		CreationOrder: order,
		TotalFiles:    totalFiles,
		TotalLines:    totalLines,
	}
}

// groupByCategory groups files by their category
func (s *PRSplitter) groupByCategory(files []claude.FileChange) map[PRCategory][]claude.FileChange {
	groups := make(map[PRCategory][]claude.FileChange)

	for _, file := range files {
		category := s.categorizeFile(file.Path)
		groups[category] = append(groups[category], file)
	}

	return groups
}

// categorizeFile determines the category of a file
// Tests are NOT treated as a special category - they stay with their implementation
func (s *PRSplitter) categorizeFile(path string) PRCategory {
	// Priority order matters - check most specific first
	switch {
	case strings.HasSuffix(path, ".md") || strings.HasSuffix(path, ".txt"):
		return CategoryDocs
	case strings.Contains(path, "cmd/"):
		return CategoryCmd
	case strings.Contains(path, "internal/"):
		return CategoryInternal
	default:
		return CategoryCore
	}
}

// estimateTotalLines estimates total lines changed (rough approximation)
func (s *PRSplitter) estimateTotalLines(files []claude.FileChange) int {
	total := 0
	for _, file := range files {
		// Rough estimate: count newlines in content
		lines := strings.Count(file.Content, "\n") + 1
		total += lines
	}
	return total
}

// shouldSplit determines if changes should be split
func (s *PRSplitter) shouldSplit(totalFiles, totalLines int, groups map[PRCategory][]claude.FileChange) bool {
	// Split if:
	// 1. Total files > maxFilesPerPR, OR
	// 2. Total lines > maxLinesPerPR

	if totalFiles > s.maxFilesPerPR {
		log.Printf("[Splitter] Exceeds max files: %d > %d", totalFiles, s.maxFilesPerPR)
		return true
	}

	if totalLines > s.maxLinesPerPR {
		log.Printf("[Splitter] Exceeds max lines: %d > %d", totalLines, s.maxLinesPerPR)
		return true
	}

	return false
}

// createSinglePRPlan creates a plan with a single PR
func (s *PRSplitter) createSinglePRPlan(files []claude.FileChange, totalFiles, totalLines int) *SplitPlan {
	subPR := SubPR{
		Index:       0,
		Name:        "Implement changes",
		Description: "All changes in a single PR",
		Files:       files,
		Category:    CategoryCore,
		DependsOn:   []int{},
		Priority:    1,
	}

	return &SplitPlan{
		SubPRs:        []SubPR{subPR},
		CreationOrder: []int{0},
		TotalFiles:    totalFiles,
		TotalLines:    totalLines,
	}
}

// generateSubPRs generates sub-PRs from categorized groups
func (s *PRSplitter) generateSubPRs(groups map[PRCategory][]claude.FileChange) []SubPR {
	var subPRs []SubPR
	index := 0

	// Process in priority order (docs, internal, core, cmd)
	// Tests are NOT a separate category - they stay with their implementation
	categories := []PRCategory{CategoryDocs, CategoryInternal, CategoryCore, CategoryCmd}

	for _, category := range categories {
		files, exists := groups[category]
		if !exists || len(files) == 0 {
			continue
		}

		// If this category group is too large, split it further
		if len(files) > s.maxFilesPerPR {
			subGroups := s.splitLargeGroup(category, files)
			for i, subGroup := range subGroups {
				subPR := s.createSubPR(index, category, subGroup, i+1)
				subPRs = append(subPRs, subPR)
				index++
			}
		} else {
			subPR := s.createSubPR(index, category, files, 0)
			subPRs = append(subPRs, subPR)
			index++
		}
	}

	// Set dependencies
	s.setDependencies(subPRs)

	return subPRs
}

// splitLargeGroup splits a large category group into smaller sub-groups
func (s *PRSplitter) splitLargeGroup(category PRCategory, files []claude.FileChange) [][]claude.FileChange {
	// Simple chunking strategy: split into chunks of maxFilesPerPR
	var subGroups [][]claude.FileChange

	for i := 0; i < len(files); i += s.maxFilesPerPR {
		end := i + s.maxFilesPerPR
		if end > len(files) {
			end = len(files)
		}
		subGroups = append(subGroups, files[i:end])
	}

	return subGroups
}

// createSubPR creates a sub-PR from a category and files
func (s *PRSplitter) createSubPR(index int, category PRCategory, files []claude.FileChange, partNum int) SubPR {
	name := s.generateSubPRName(category, len(files), partNum)
	description := s.generateSubPRDescription(category, files)
	priority := s.getCategoryPriority(category)

	return SubPR{
		Index:       index,
		Name:        name,
		Description: description,
		Files:       files,
		Category:    category,
		DependsOn:   []int{}, // Will be set by setDependencies
		Priority:    priority,
	}
}

// generateSubPRName generates a human-readable name for a sub-PR
func (s *PRSplitter) generateSubPRName(category PRCategory, fileCount int, partNum int) string {
	suffix := ""
	if partNum > 0 {
		suffix = fmt.Sprintf(" (part %d)", partNum)
	}

	switch category {
	case CategoryTests:
		return fmt.Sprintf("Add test infrastructure%s", suffix)
	case CategoryDocs:
		return fmt.Sprintf("Update documentation%s", suffix)
	case CategoryInternal:
		return fmt.Sprintf("Add internal infrastructure%s", suffix)
	case CategoryCore:
		return fmt.Sprintf("Implement core functionality%s", suffix)
	case CategoryCmd:
		return fmt.Sprintf("Update command line interface%s", suffix)
	default:
		return fmt.Sprintf("Add changes%s", suffix)
	}
}

// generateSubPRDescription generates a detailed description for a sub-PR
func (s *PRSplitter) generateSubPRDescription(category PRCategory, files []claude.FileChange) string {
	var lines []string

	// Add category header
	categoryName := s.getCategoryDisplayName(category)
	lines = append(lines, fmt.Sprintf("## %s", categoryName))
	lines = append(lines, "")

	// Calculate total lines
	totalLines := 0

	// Add file-level details with line counts
	lines = append(lines, "### Files Changed")
	for _, file := range files {
		lineCount := strings.Count(file.Content, "\n") + 1
		totalLines += lineCount
		lines = append(lines, fmt.Sprintf("- `%s` (%d lines)", file.Path, lineCount))
	}

	lines = append(lines, "")
	lines = append(lines, fmt.Sprintf("**Summary:** %d files, ~%d lines", len(files), totalLines))
	lines = append(lines, "")
	lines = append(lines, "**Context:** This PR is part of a larger change. See the full task description in the original issue.")

	return strings.Join(lines, "\n")
}

// getCategoryPriority returns the priority for a category (lower = earlier)
func (s *PRSplitter) getCategoryPriority(category PRCategory) int {
	priorities := map[PRCategory]int{
		CategoryTests:    1, // Tests and docs are independent, create first
		CategoryDocs:     1,
		CategoryInternal: 2, // Internal infrastructure depends on tests
		CategoryCore:     3, // Core depends on internal
		CategoryCmd:      4, // Cmd depends on core
	}

	if p, ok := priorities[category]; ok {
		return p
	}
	return 5 // Unknown categories last
}

// getCategoryDisplayName returns human-readable category name
func (s *PRSplitter) getCategoryDisplayName(category PRCategory) string {
	switch category {
	case CategoryTests:
		return "Test Infrastructure"
	case CategoryDocs:
		return "Documentation Updates"
	case CategoryInternal:
		return "Internal Infrastructure"
	case CategoryCore:
		return "Core Functionality"
	case CategoryCmd:
		return "Command Line Interface"
	default:
		return "Changes"
	}
}

// setDependencies sets dependency relationships between sub-PRs
func (s *PRSplitter) setDependencies(subPRs []SubPR) {
	// Build a map of category to sub-PR indices
	categoryMap := make(map[PRCategory][]int)
	for i := range subPRs {
		category := subPRs[i].Category
		categoryMap[category] = append(categoryMap[category], i)
	}

	// Set dependencies based on priority rules:
	// - Core depends on Internal (if internal exists)
	// - Cmd depends on Core (if core exists)

	internalIndices := categoryMap[CategoryInternal]
	coreIndices := categoryMap[CategoryCore]

	// Core depends on internal
	if len(internalIndices) > 0 {
		for _, idx := range coreIndices {
			subPRs[idx].DependsOn = append(subPRs[idx].DependsOn, internalIndices...)
		}
	}

	// Cmd depends on core
	cmdIndices := categoryMap[CategoryCmd]
	if len(coreIndices) > 0 {
		for _, idx := range cmdIndices {
			subPRs[idx].DependsOn = append(subPRs[idx].DependsOn, coreIndices...)
		}
	}
}

// determineCreationOrder determines the order to create PRs
// Independent PRs (no dependencies) come first, then dependent PRs
func (s *PRSplitter) determineCreationOrder(subPRs []SubPR) []int {
	var order []int

	// Phase 1: Add all independent sub-PRs (no dependencies)
	for i := range subPRs {
		if len(subPRs[i].DependsOn) == 0 {
			order = append(order, i)
		}
	}

	// Phase 2: Add dependent sub-PRs sorted by priority
	// (In a real implementation, these would be created after dependencies are merged)
	for i := range subPRs {
		if len(subPRs[i].DependsOn) > 0 {
			order = append(order, i)
		}
	}

	return order
}
