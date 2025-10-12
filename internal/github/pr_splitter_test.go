package github

import (
	"strings"
	"testing"

	"github.com/cexll/swe/internal/provider/claude"
)

// Helper function to create test file changes
func createTestFiles(paths ...string) []claude.FileChange {
	files := make([]claude.FileChange, len(paths))
	for i, path := range paths {
		files[i] = claude.FileChange{
			Path:    path,
			Content: "test content\n",
		}
	}
	return files
}

// TestNewPRSplitter tests PRSplitter creation
func TestNewPRSplitter(t *testing.T) {
	tests := []struct {
		name          string
		maxFiles      int
		maxLines      int
		expectedFiles int
		expectedLines int
	}{
		{
			name:          "default values",
			maxFiles:      0,
			maxLines:      0,
			expectedFiles: 8,
			expectedLines: 300,
		},
		{
			name:          "custom values",
			maxFiles:      10,
			maxLines:      500,
			expectedFiles: 10,
			expectedLines: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			splitter := NewPRSplitter(tt.maxFiles, tt.maxLines)
			if splitter.maxFilesPerPR != tt.expectedFiles {
				t.Errorf("expected maxFilesPerPR=%d, got %d", tt.expectedFiles, splitter.maxFilesPerPR)
			}
			if splitter.maxLinesPerPR != tt.expectedLines {
				t.Errorf("expected maxLinesPerPR=%d, got %d", tt.expectedLines, splitter.maxLinesPerPR)
			}
		})
	}
}

// TestCategorizeFile tests file categorization
func TestCategorizeFile(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	tests := []struct {
		path             string
		expectedCategory PRCategory
	}{
		{"internal/provider/claude_test.go", CategoryInternal}, // Tests follow their implementation
		{"cmd/main.go", CategoryCmd},
		{"internal/github/auth.go", CategoryInternal},
		{"README.md", CategoryDocs},
		{"CHANGELOG.txt", CategoryDocs},
		{"pkg/util/helper.go", CategoryCore},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			category := splitter.categorizeFile(tt.path)
			if category != tt.expectedCategory {
				t.Errorf("expected category=%s, got %s", tt.expectedCategory, category)
			}
		})
	}
}

// TestAnalyze_SmallPR_NoIntelligentSplit tests very small PRs without intelligent split
func TestAnalyze_SmallPR_NoIntelligentSplit(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	// Create 3 files (below threshold, all same category - no intelligent split)
	files := createTestFiles(
		"internal/auth.go",
		"internal/client.go",
		"internal/helper.go",
	)

	plan := splitter.Analyze(files, "test prompt")

	// Should result in a single PR (no intelligent split with single category)
	if len(plan.SubPRs) != 1 {
		t.Errorf("expected 1 sub-PR, got %d", len(plan.SubPRs))
	}

	if plan.TotalFiles != 3 {
		t.Errorf("expected TotalFiles=3, got %d", plan.TotalFiles)
	}
}

// TestAnalyze_LargePR_AutoSplit tests that large PRs are automatically split
func TestAnalyze_LargePR_AutoSplit(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	// Create 15 files (above threshold)
	files := createTestFiles(
		"internal/auth_test.go",
		"internal/client_test.go",
		"internal/helper_test.go",
		"internal/auth.go",
		"internal/client.go",
		"internal/helper.go",
		"internal/config.go",
		"pkg/core/handler.go",
		"pkg/core/processor.go",
		"pkg/core/validator.go",
		"cmd/main.go",
		"cmd/server.go",
		"README.md",
		"CHANGELOG.md",
		"docs/guide.md",
	)

	plan := splitter.Analyze(files, "test prompt")

	// Should result in multiple sub-PRs
	if len(plan.SubPRs) <= 1 {
		t.Errorf("expected multiple sub-PRs, got %d", len(plan.SubPRs))
	}

	if plan.TotalFiles != 15 {
		t.Errorf("expected TotalFiles=15, got %d", plan.TotalFiles)
	}

	// Verify that categories are properly split
	// Tests are now grouped with their implementation (internal/core/cmd)
	hasDocs := false
	hasCore := false

	for _, subPR := range plan.SubPRs {
		switch subPR.Category {
		case CategoryDocs:
			hasDocs = true
		case CategoryCore, CategoryInternal, CategoryCmd:
			hasCore = true
		}
	}

	if !hasDocs {
		t.Error("expected docs category in split plan")
	}
	if !hasCore {
		t.Error("expected core/internal/cmd category in split plan")
	}
}

// TestAnalyze_NoIntelligentSplit tests that tests stay with implementation
func TestAnalyze_NoIntelligentSplit(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	// Create 6 files: tests + core (below threshold, should NOT split)
	// Tests stay with their implementation - no intelligent split
	files := createTestFiles(
		"internal/auth_test.go",
		"internal/client_test.go",
		"internal/auth.go",
		"internal/client.go",
		"internal/config.go",
		"README.md",
	)

	plan := splitter.Analyze(files, "test prompt")

	// Should result in a single PR (tests + implementation together)
	if len(plan.SubPRs) != 1 {
		t.Errorf("expected single PR (no intelligent split), got %d sub-PRs", len(plan.SubPRs))
	}

	// Verify tests and implementation are together
	if len(plan.SubPRs) > 0 {
		subPR := plan.SubPRs[0]
		// Should include both test files and implementation files
		if len(subPR.Files) != 6 {
			t.Errorf("expected all 6 files in single PR, got %d", len(subPR.Files))
		}
	}
}

// TestCategoryPriority tests that categories have correct priorities
func TestCategoryPriority(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	tests := []struct {
		category         PRCategory
		expectedPriority int
	}{
		{CategoryTests, 1},
		{CategoryDocs, 1},
		{CategoryInternal, 2},
		{CategoryCore, 3},
		{CategoryCmd, 4},
	}

	for _, tt := range tests {
		t.Run(string(tt.category), func(t *testing.T) {
			priority := splitter.getCategoryPriority(tt.category)
			if priority != tt.expectedPriority {
				t.Errorf("expected priority=%d, got %d", tt.expectedPriority, priority)
			}
		})
	}
}

// TestDependencyTracking tests that dependencies are correctly set
func TestDependencyTracking(t *testing.T) {
	splitter := NewPRSplitter(5, 300)

	// Create files that should result in dependencies:
	// Internal (includes tests) → Core (depends on internal)
	files := createTestFiles(
		"internal/auth_test.go",
		"internal/client_test.go",
		"internal/auth.go",
		"internal/client.go",
		"pkg/core/handler.go",
		"pkg/core/processor.go",
	)

	plan := splitter.Analyze(files, "test prompt")

	// Find the sub-PRs by category
	var internalIdx, coreIdx int
	internalFound, coreFound := false, false

	for i, subPR := range plan.SubPRs {
		switch subPR.Category {
		case CategoryInternal:
			internalIdx = i
			internalFound = true
		case CategoryCore:
			coreIdx = i
			coreFound = true
		}
	}

	if !internalFound || !coreFound {
		t.Fatal("expected internal and core sub-PRs")
	}

	// Internal should have no dependencies (tests are included with it)
	if len(plan.SubPRs[internalIdx].DependsOn) != 0 {
		t.Errorf("internal should have no dependencies, got %v", plan.SubPRs[internalIdx].DependsOn)
	}

	// Core should depend on internal
	if len(plan.SubPRs[coreIdx].DependsOn) == 0 {
		t.Error("core should depend on internal")
	}
}

// TestCreationOrder tests that PRs are ordered correctly for creation
func TestCreationOrder(t *testing.T) {
	splitter := NewPRSplitter(5, 300)

	files := createTestFiles(
		"internal/auth_test.go",
		"internal/auth.go",
		"pkg/core/handler.go",
		"README.md",
	)

	plan := splitter.Analyze(files, "test prompt")

	// First items in creation order should be independent (tests, docs)
	if len(plan.CreationOrder) == 0 {
		t.Fatal("expected non-empty creation order")
	}

	firstIdx := plan.CreationOrder[0]
	firstSubPR := plan.SubPRs[firstIdx]

	// First PR should have no dependencies
	if len(firstSubPR.DependsOn) != 0 {
		t.Errorf("first PR in creation order should have no dependencies, got %v", firstSubPR.DependsOn)
	}
}

// TestSplitLargeGroup tests that large category groups are split
func TestSplitLargeGroup(t *testing.T) {
	splitter := NewPRSplitter(3, 300)

	// Create 10 test files in internal/ (will exceed maxFilesPerPR for internal category)
	// Tests are now grouped with their implementation (internal)
	files := createTestFiles(
		"internal/auth_test.go",
		"internal/client_test.go",
		"internal/config_test.go",
		"internal/handler_test.go",
		"internal/processor_test.go",
		"internal/validator_test.go",
		"internal/helper_test.go",
		"internal/util_test.go",
		"internal/cache_test.go",
		"internal/queue_test.go",
	)

	plan := splitter.Analyze(files, "test prompt")

	// Should split internal (which includes tests) into multiple sub-PRs
	internalSubPRs := 0
	for _, subPR := range plan.SubPRs {
		if subPR.Category == CategoryInternal {
			internalSubPRs++
			// Each internal sub-PR should have <= maxFilesPerPR files
			if len(subPR.Files) > splitter.maxFilesPerPR {
				t.Errorf("internal sub-PR has %d files, exceeds max %d", len(subPR.Files), splitter.maxFilesPerPR)
			}
		}
	}

	if internalSubPRs <= 1 {
		t.Errorf("expected multiple internal sub-PRs, got %d", internalSubPRs)
	}
}

// TestGenerateSubPRName tests sub-PR name generation
func TestGenerateSubPRName(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	tests := []struct {
		category     PRCategory
		fileCount    int
		partNum      int
		expectedName string
	}{
		{CategoryTests, 3, 0, "Add test infrastructure"},
		{CategoryTests, 3, 1, "Add test infrastructure (part 1)"},
		{CategoryDocs, 2, 0, "Update documentation"},
		{CategoryInternal, 5, 0, "Add internal infrastructure"},
		{CategoryCore, 8, 0, "Implement core functionality"},
		{CategoryCmd, 2, 0, "Update command line interface"},
	}

	for _, tt := range tests {
		t.Run(tt.expectedName, func(t *testing.T) {
			name := splitter.generateSubPRName(tt.category, tt.fileCount, tt.partNum)
			if name != tt.expectedName {
				t.Errorf("expected name=%q, got %q", tt.expectedName, name)
			}
		})
	}
}

// TestEstimateTotalLines tests line count estimation
func TestEstimateTotalLines(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	files := []claude.FileChange{
		{Path: "file1.go", Content: "line1\nline2\nline3"},
		{Path: "file2.go", Content: "line1\nline2"},
		{Path: "file3.go", Content: "single line"},
	}

	totalLines := splitter.estimateTotalLines(files)

	// file1: 3 lines, file2: 2 lines, file3: 1 line = 6 total
	expectedLines := 6
	if totalLines != expectedLines {
		t.Errorf("expected %d lines, got %d", expectedLines, totalLines)
	}
}

// TestGroupByCategory tests file grouping by category
func TestGroupByCategory(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	files := createTestFiles(
		"internal/auth_test.go",
		"internal/auth.go",
		"cmd/main.go",
		"README.md",
		"pkg/handler.go",
	)

	groups := splitter.groupByCategory(files)

	// Verify expected number of categories (tests are grouped with internal)
	expectedCategories := 4 // internal (includes test), cmd, docs, core
	if len(groups) != expectedCategories {
		t.Errorf("expected %d categories, got %d", expectedCategories, len(groups))
	}

	// Verify each category has correct files
	if len(groups[CategoryInternal]) != 2 { // auth_test.go and auth.go
		t.Errorf("expected 2 internal files (including test), got %d", len(groups[CategoryInternal]))
	}
	if len(groups[CategoryCmd]) != 1 {
		t.Errorf("expected 1 cmd file, got %d", len(groups[CategoryCmd]))
	}
	if len(groups[CategoryDocs]) != 1 {
		t.Errorf("expected 1 docs file, got %d", len(groups[CategoryDocs]))
	}
	if len(groups[CategoryCore]) != 1 {
		t.Errorf("expected 1 core file, got %d", len(groups[CategoryCore]))
	}
}

// ===== Phase 3: Coverage Gap Tests =====

// TestAnalyze_ExceedsLineThreshold tests splitting based on line count threshold
func TestAnalyze_ExceedsLineThreshold(t *testing.T) {
	splitter := NewPRSplitter(10, 50) // Small line threshold for testing

	// Create files with different categories and lots of lines (total > 50 lines)
	files := []claude.FileChange{
		{Path: "internal/auth_test.go", Content: strings.Repeat("line\n", 15)}, // 15 lines, tests
		{Path: "internal/auth.go", Content: strings.Repeat("line\n", 12)},      // 12 lines, internal
		{Path: "pkg/core.go", Content: strings.Repeat("line\n", 10)},           // 10 lines, core
		{Path: "cmd/main.go", Content: strings.Repeat("line\n", 8)},            // 8 lines, cmd
		{Path: "README.md", Content: strings.Repeat("line\n", 8)},              // 8 lines, docs
	}
	// Total: 53 lines (exceeds threshold of 50)

	plan := splitter.Analyze(files, "test prompt")

	// Should trigger split due to line count
	if len(plan.SubPRs) <= 1 {
		t.Errorf("expected split due to line threshold, got %d sub-PRs", len(plan.SubPRs))
	}

	if plan.TotalLines <= 50 {
		t.Errorf("expected total lines > 50, got %d", plan.TotalLines)
	}
}

// TestGetCategoryPriority_UnknownCategory tests handling of unknown categories
func TestGetCategoryPriority_UnknownCategory(t *testing.T) {
	splitter := NewPRSplitter(8, 300)

	// Test unknown category returns priority 5
	unknownCategory := PRCategory("unknown_category")
	priority := splitter.getCategoryPriority(unknownCategory)

	expectedPriority := 5
	if priority != expectedPriority {
		t.Errorf("unknown category should have priority=%d, got %d", expectedPriority, priority)
	}

	// Verify it's lower priority than all known categories
	for _, knownCategory := range []PRCategory{CategoryTests, CategoryDocs, CategoryInternal, CategoryCore, CategoryCmd} {
		knownPriority := splitter.getCategoryPriority(knownCategory)
		if priority <= knownPriority {
			t.Errorf("unknown category priority (%d) should be > known category %s priority (%d)",
				priority, knownCategory, knownPriority)
		}
	}
}
