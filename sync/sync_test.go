package sync

import (
	"fmt"
	"testing"
	"time"
)

func TestRepositoryStatus_String(t *testing.T) {
	tests := []struct {
		status   RepositoryStatus
		expected string
	}{
		{StatusPending, "Pending"},
		{StatusCloning, "Cloning..."},
		{StatusFetching, "Fetching..."},
		{StatusCompleted, "✓ Completed"},
		{StatusFailed, "✗ Failed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.status.String(); got != tt.expected {
				t.Errorf("RepositoryStatus.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestDefaultSyncConfig(t *testing.T) {
	config := DefaultSyncConfig()

	if config.MaxConcurrency <= 0 {
		t.Errorf("MaxConcurrency should be positive, got %d", config.MaxConcurrency)
	}

	if config.Timeout <= 0 {
		t.Errorf("Timeout should be positive, got %v", config.Timeout)
	}

	if config.RetryAttempts < 0 {
		t.Errorf("RetryAttempts should be non-negative, got %d", config.RetryAttempts)
	}
}

func TestRepoExists(t *testing.T) {
	// Test with current directory (should have .git since this is a git repo)
	if !repoExists(".") {
		t.Skip("Skipping test - current directory is not a git repository")
	}

	// Test with non-existent directory
	if repoExists("/non/existent/path") {
		t.Error("Non-existent path should not be detected as a git repository")
	}
}

func TestIsNonRetryableError(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"authentication error", &mockError{"authentication failed"}, true},
		{"permission error", &mockError{"permission denied"}, true},
		{"not found error", &mockError{"repository not found"}, true},
		{"network error", &mockError{"network timeout"}, false},
		{"generic error", &mockError{"something went wrong"}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isNonRetryableError(tt.err); got != tt.expected {
				t.Errorf("isNonRetryableError() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestModel_UpdateRepositoryStatus(t *testing.T) {
	model := NewModel("test-org")
	model.Repositories = []Repository{
		{Name: "repo1", Status: StatusPending},
		{Name: "repo2", Status: StatusPending},
	}

	// Update first repository
	model.updateRepositoryStatus("repo1", StatusCloning, nil)

	if model.Repositories[0].Status != StatusCloning {
		t.Errorf("Expected repo1 status to be StatusCloning, got %v", model.Repositories[0].Status)
	}

	if model.Repositories[0].StartTime.IsZero() {
		t.Error("Expected StartTime to be set for StatusCloning")
	}

	// Update to completed
	model.updateRepositoryStatus("repo1", StatusCompleted, nil)

	if model.Repositories[0].Status != StatusCompleted {
		t.Errorf("Expected repo1 status to be StatusCompleted, got %v", model.Repositories[0].Status)
	}

	if model.Repositories[0].EndTime.IsZero() {
		t.Error("Expected EndTime to be set for StatusCompleted")
	}
}

func TestModel_CountCompleted(t *testing.T) {
	model := NewModel("test-org")
	model.Repositories = []Repository{
		{Name: "repo1", Status: StatusCompleted},
		{Name: "repo2", Status: StatusFailed},
		{Name: "repo3", Status: StatusPending},
		{Name: "repo4", Status: StatusCloning},
	}

	completed := model.countCompleted()
	expected := 2 // StatusCompleted + StatusFailed

	if completed != expected {
		t.Errorf("Expected %d completed repositories, got %d", expected, completed)
	}
}

func TestModel_FormatDuration(t *testing.T) {
	model := NewModel("test-org")

	// Test with no start time
	repo := Repository{Name: "test"}
	duration := model.formatDuration(repo)
	if duration != "-" {
		t.Errorf("Expected '-' for no start time, got %s", duration)
	}

	// Test with start time but no end time (ongoing)
	repo.StartTime = time.Now().Add(-5 * time.Second)
	duration = model.formatDuration(repo)
	if duration == "-" {
		t.Error("Expected duration for ongoing operation")
	}

	// Test with both start and end time
	repo.EndTime = repo.StartTime.Add(3 * time.Second)
	duration = model.formatDuration(repo)
	if duration != "3s" {
		t.Errorf("Expected '3s', got %s", duration)
	}
}

// Mock error for testing
type mockError struct {
	message string
}

func (e *mockError) Error() string {
	return e.message
}

// Benchmark to ensure performance is reasonable
func BenchmarkUpdateRepositoryStatus(b *testing.B) {
	model := NewModel("test-org")

	// Create many repositories
	repos := make([]Repository, 1000)
	for i := range repos {
		repos[i] = Repository{Name: fmt.Sprintf("repo%d", i), Status: StatusPending}
	}
	model.Repositories = repos

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		model.updateRepositoryStatus("repo500", StatusCompleted, nil)
	}
}
