package sync

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Repository represents a repository and its sync status
type Repository struct {
	Name      string
	Status    RepositoryStatus
	Error     error
	StartTime time.Time
	EndTime   time.Time
}

type RepositoryStatus int

const (
	StatusPending RepositoryStatus = iota
	StatusCloning
	StatusFetching
	StatusCompleted
	StatusFailed
)

func (s RepositoryStatus) String() string {
	switch s {
	case StatusPending:
		return "Pending"
	case StatusCloning:
		return "Cloning..."
	case StatusFetching:
		return "Fetching..."
	case StatusCompleted:
		return "✓ Completed"
	case StatusFailed:
		return "✗ Failed"
	default:
		return "Unknown"
	}
}

// SyncConfig holds configuration for the sync operation
type SyncConfig struct {
	MaxConcurrency int
	Timeout        time.Duration
	RetryAttempts  int
	RetryDelay     time.Duration
}

func DefaultSyncConfig() SyncConfig {
	return SyncConfig{
		MaxConcurrency: 5, // Limit concurrent operations
		Timeout:        30 * time.Second,
		RetryAttempts:  2,
		RetryDelay:     1 * time.Second,
	}
}

// Model represents the TUI model
type Model struct {
	Org          string
	Repositories []Repository
	Config       SyncConfig
	Done         bool
	Progress     progress.Model
	Spinner      spinner.Model
	Table        table.Model
	Width        int
	Height       int
	ctx          context.Context
	cancel       context.CancelFunc
	statusChan   chan repositoryStatusMsg
}

const (
	padding  = 2
	maxWidth = 80
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFDD00")).Background(lipgloss.Color("#336699"))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")) // Orange
	activeStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("#00BFFF")) // Blue
	successStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")) // Green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")) // Red
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	normalText   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
)

func NewModel(org string) Model {
	ctx, cancel := context.WithCancel(context.Background())

	progressBar := progress.New(progress.WithDefaultGradient(), progress.WithScaledGradient("#FFA500", "#00FF00"))
	spn := spinner.New()
	spn.Style = spinnerStyle

	columns := []table.Column{
		{Title: "Repository", Width: 30},
		{Title: "Status", Width: 20},
		{Title: "Duration", Width: 10},
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithHeight(15),
	)

	return Model{
		Org:        org,
		Config:     DefaultSyncConfig(),
		Progress:   progressBar,
		Spinner:    spn,
		Table:      tbl,
		ctx:        ctx,
		cancel:     cancel,
		statusChan: make(chan repositoryStatusMsg, 100),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchRepositories, m.Spinner.Tick, m.listenForUpdates())
}

// Listen for repository updates
func (m Model) listenForUpdates() tea.Cmd {
	return func() tea.Msg {
		select {
		case update, ok := <-m.statusChan:
			if !ok {
				return nil // Channel closed
			}
			return update
		case <-m.ctx.Done():
			return nil
		}
	}
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			m.cancel() // Cancel ongoing operations
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.Width = msg.Width
		m.Height = msg.Height
		m.Progress.Width = msg.Width - padding*2 - 4
		if m.Progress.Width > maxWidth {
			m.Progress.Width = maxWidth
		}
		return m, nil
	case repositoriesFetchedMsg:
		m.Repositories = msg.Repositories
		m.updateTable()
		return m, m.syncRepositories()
	case repositoryStatusMsg:
		m.updateRepositoryStatus(msg.Name, msg.Status, msg.Error)
		m.updateTable()

		completed := m.countCompleted()
		total := len(m.Repositories)

		if completed == total {
			m.Done = true
			return m, m.Progress.SetPercent(1.0)
		}

		return m, tea.Batch(m.Progress.SetPercent(float64(completed)/float64(total)), m.listenForUpdates())
	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		progressModel, cmd := m.Progress.Update(msg)
		m.Progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

func (m Model) View() string {
	var builder strings.Builder
	title := titleStyle.Render("OrgSync")
	orgInfo := normalText.Render(fmt.Sprintf("Organization: %s", m.Org))
	progressBar := m.Progress.View()

	center := func(s string) string {
		return lipgloss.Place(m.Width, len(strings.Split(s, "\n")), lipgloss.Center, lipgloss.Center, s)
	}

	builder.WriteString(center(title) + "\n\n")
	builder.WriteString(center(orgInfo) + "\n\n")
	builder.WriteString(center(progressBar) + "\n\n")

	if m.Done {
		summary := m.generateSummary()
		builder.WriteString(center(summary) + "\n\n")
		builder.WriteString(center("Press 'q' to quit.") + "\n")
	} else {
		loadingSpinner := m.Spinner.View() + " Syncing repositories..."
		builder.WriteString(center(loadingSpinner) + "\n\n")
		builder.WriteString(center(m.Table.View()) + "\n\n")
		builder.WriteString(center("Press 'q' or Ctrl+C to cancel.") + "\n")
	}

	return builder.String()
}

// Message types
type repositoriesFetchedMsg struct {
	Repositories []Repository
}

type repositoryStatusMsg struct {
	Name   string
	Status RepositoryStatus
	Error  error
}

// Helper methods
func (m *Model) updateRepositoryStatus(name string, status RepositoryStatus, err error) {
	for i := range m.Repositories {
		if m.Repositories[i].Name == name {
			m.Repositories[i].Status = status
			m.Repositories[i].Error = err

			if status == StatusCloning || status == StatusFetching {
				m.Repositories[i].StartTime = time.Now()
			} else if status == StatusCompleted || status == StatusFailed {
				m.Repositories[i].EndTime = time.Now()
			}
			break
		}
	}
}

func (m *Model) updateTable() {
	rows := make([]table.Row, 0, len(m.Repositories))

	for _, repo := range m.Repositories {
		if repo.Status == StatusCompleted {
			continue // Hide completed repos to reduce clutter
		}

		statusText := m.formatStatus(repo)
		duration := m.formatDuration(repo)

		rows = append(rows, table.Row{repo.Name, statusText, duration})
	}

	m.Table.SetRows(rows)
}

func (m *Model) formatStatus(repo Repository) string {
	switch repo.Status {
	case StatusPending:
		return pendingStyle.Render(repo.Status.String())
	case StatusCloning, StatusFetching:
		return activeStyle.Render(repo.Status.String())
	case StatusCompleted:
		return successStyle.Render(repo.Status.String())
	case StatusFailed:
		errorText := repo.Status.String()
		if repo.Error != nil {
			errorText += fmt.Sprintf(" (%s)", repo.Error.Error())
		}
		return errorStyle.Render(errorText)
	default:
		return repo.Status.String()
	}
}

func (m *Model) formatDuration(repo Repository) string {
	if repo.StartTime.IsZero() {
		return "-"
	}

	end := repo.EndTime
	if end.IsZero() {
		end = time.Now()
	}

	duration := end.Sub(repo.StartTime)
	if duration < time.Second {
		return "<1s"
	}
	return duration.Truncate(time.Second).String()
}

func (m *Model) countCompleted() int {
	count := 0
	for _, repo := range m.Repositories {
		if repo.Status == StatusCompleted || repo.Status == StatusFailed {
			count++
		}
	}
	return count
}

func (m *Model) generateSummary() string {
	total := len(m.Repositories)
	completed := 0
	failed := 0

	for _, repo := range m.Repositories {
		switch repo.Status {
		case StatusCompleted:
			completed++
		case StatusFailed:
			failed++
		}
	}

	summary := fmt.Sprintf("Sync completed: %d/%d successful", completed, total)
	if failed > 0 {
		summary += fmt.Sprintf(", %d failed", failed)
	}

	return summary
}

// Business logic
func (m Model) fetchRepositories() tea.Msg {
	repos, err := fetchReposInOrg(m.ctx, m.Org)
	if err != nil {
		return repositoriesFetchedMsg{
			Repositories: []Repository{{Name: "Error fetching repos", Status: StatusFailed, Error: err}},
		}
	}

	repositories := make([]Repository, len(repos))
	for i, repo := range repos {
		repositories[i] = Repository{
			Name:   repo,
			Status: StatusPending,
		}
	}

	return repositoriesFetchedMsg{Repositories: repositories}
}

func (m Model) syncRepositories() tea.Cmd {
	return func() tea.Msg {
		// Use a semaphore to limit concurrency
		semaphore := make(chan struct{}, m.Config.MaxConcurrency)
		var wg sync.WaitGroup

		for _, repo := range m.Repositories {
			wg.Add(1)
			go func(r Repository) {
				defer wg.Done()

				// Acquire semaphore
				select {
				case semaphore <- struct{}{}:
					defer func() { <-semaphore }()
				case <-m.ctx.Done():
					return
				}

				// Send cloning status first
				select {
				case m.statusChan <- repositoryStatusMsg{
					Name:   r.Name,
					Status: StatusCloning,
					Error:  nil,
				}:
				case <-m.ctx.Done():
					return
				}

				// Sync the repository with retries
				err := m.syncRepoWithRetry(r.Name)

				status := StatusCompleted
				if err != nil {
					status = StatusFailed
				}

				// Send final status
				select {
				case m.statusChan <- repositoryStatusMsg{
					Name:   r.Name,
					Status: status,
					Error:  err,
				}:
				case <-m.ctx.Done():
					return
				}
			}(repo)
		}

		// Close status channel when all goroutines complete
		go func() {
			wg.Wait()
			close(m.statusChan)
		}()

		// Don't return anything - let the listener handle updates
		return nil
	}
}

func (m Model) syncRepoWithRetry(repoName string) error {
	var lastErr error

	for attempt := 0; attempt <= m.Config.RetryAttempts; attempt++ {
		if attempt > 0 {
			select {
			case <-time.After(m.Config.RetryDelay):
			case <-m.ctx.Done():
				return m.ctx.Err()
			}
		}

		err := m.syncRepo(repoName)
		if err == nil {
			return nil
		}

		lastErr = err

		// Don't retry certain types of errors
		if isNonRetryableError(err) {
			break
		}
	}

	return lastErr
}

func (m Model) syncRepo(repoName string) error {
	repoDir := filepath.Join(".", repoName)

	ctx, cancel := context.WithTimeout(m.ctx, m.Config.Timeout)
	defer cancel()

	if repoExists(repoDir) {
		return fetchRepo(ctx, repoDir, repoName)
	}
	return cloneRepo(ctx, m.Org, repoName, repoDir)
}

// Utility functions
func fetchReposInOrg(ctx context.Context, org string) ([]string, error) {
	cmd := exec.CommandContext(ctx, "gh", "repo", "list", org, "--json", "name", "--jq", ".[] | .name", "--limit", "1000")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to fetch repos for org %s: %w", org, err)
	}

	output := strings.TrimSpace(out.String())
	if output == "" {
		return []string{}, nil
	}

	repos := strings.Split(output, "\n")
	return repos, nil
}

func repoExists(repoDir string) bool {
	gitDir := filepath.Join(repoDir, ".git")
	_, err := os.Stat(gitDir)
	return !os.IsNotExist(err)
}

func cloneRepo(ctx context.Context, org, repo, repoDir string) error {
	cmd := exec.CommandContext(ctx, "gh", "repo", "clone", fmt.Sprintf("%s/%s", org, repo), repoDir)

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone %s/%s: %w (%s)", org, repo, err, stderr.String())
	}
	return nil
}

func fetchRepo(ctx context.Context, repoDir, repo string) error {
	cmd := exec.CommandContext(ctx, "git", "-C", repoDir, "fetch", "origin")

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch %s: %w (%s)", repo, err, stderr.String())
	}
	return nil
}

func isNonRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()
	// Don't retry authentication or permission errors
	return strings.Contains(errStr, "authentication") ||
		strings.Contains(errStr, "permission denied") ||
		strings.Contains(errStr, "not found") ||
		strings.Contains(errStr, "repository not found")
}
