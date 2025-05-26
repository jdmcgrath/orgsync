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
		return "⏳ Pending"
	case StatusCloning:
		return "📥 Cloning"
	case StatusFetching:
		return "🔄 Fetching"
	case StatusCompleted:
		return "✅ Completed"
	case StatusFailed:
		return "❌ Failed"
	default:
		return "❓ Unknown"
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
	padding        = 2
	minWidth       = 80
	maxWidth       = 120
	minColRepo     = 25
	minColStatus   = 25
	minColDuration = 12
)

var (
	// Enhanced color palette with modern colors
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#667eea")).
			Padding(0, 2).
			MarginBottom(1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#764ba2"))

	orgStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0AEC0")).
			Background(lipgloss.Color("#1A202C")).
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4A5568")).
			MarginBottom(1)

	// Status styles with enhanced colors and effects
	pendingStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#F6AD55")).
			Background(lipgloss.Color("#2D2017")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#F6AD55"))

	activeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#63B3ED")).
			Background(lipgloss.Color("#1A202C")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#63B3ED")).
			Blink(true)

	successStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#68D391")).
			Background(lipgloss.Color("#1C2D1C")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#68D391"))

	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#FC8181")).
			Background(lipgloss.Color("#2D1B1B")).
			Padding(0, 1).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#FC8181"))

	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#9F7AEA")).
			Bold(true)

	normalText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))

	subtleText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0AEC0"))

	// Table styles
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#4A5568")).
				Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(lipgloss.Color("#718096"))

	tableRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")).
			Background(lipgloss.Color("#2D3748"))

	// Progress bar with custom gradient
	progressBarStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#4A5568")).
				Padding(0, 1)

	// Summary styles
	summaryStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.Color("#38A169")).
			Padding(1, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#38A169")).
			MarginTop(1)

	instructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#A0AEC0")).
				Italic(true)
)

func NewModel(org string) Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Enhanced progress bar with custom gradient
	progressBar := progress.New(
		progress.WithScaledGradient("#667eea", "#764ba2"),
		progress.WithoutPercentage(),
	)

	// Enhanced spinner
	spn := spinner.New()
	spn.Style = spinnerStyle
	spn.Spinner = spinner.Dot

	// Dynamic table columns - will be updated in window resize
	columns := []table.Column{
		{Title: "📁 Repository", Width: minColRepo},
		{Title: "🔄 Status", Width: minColStatus},
		{Title: "⏱️  Duration", Width: minColDuration},
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithHeight(15),
		table.WithFocused(false),
	)

	// Enhanced table styling
	tableStyles := table.DefaultStyles()
	tableStyles.Header = tableHeaderStyle
	tableStyles.Cell = tableRowStyle
	tableStyles.Selected = tableRowStyle
	tbl.SetStyles(tableStyles)

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

		// Dynamic column sizing based on terminal width
		m.updateTableColumns()

		// Update progress bar width
		progressWidth := m.Width - padding*4
		if progressWidth > maxWidth {
			progressWidth = maxWidth
		}
		if progressWidth < minWidth {
			progressWidth = minWidth
		}
		m.Progress.Width = progressWidth

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

func (m *Model) updateTableColumns() {
	if m.Width < minWidth {
		return
	}

	// Calculate available width for table
	availableWidth := m.Width - padding*4
	if availableWidth > maxWidth {
		availableWidth = maxWidth
	}

	// Distribute width among columns
	// Repository: 40%, Status: 40%, Duration: 20%
	repoWidth := max(minColRepo, int(float64(availableWidth)*0.4))
	statusWidth := max(minColStatus, int(float64(availableWidth)*0.4))
	durationWidth := max(minColDuration, int(float64(availableWidth)*0.2))

	// Ensure we don't exceed available width
	totalWidth := repoWidth + statusWidth + durationWidth
	if totalWidth > availableWidth {
		// Scale down proportionally
		scale := float64(availableWidth) / float64(totalWidth)
		repoWidth = max(minColRepo, int(float64(repoWidth)*scale))
		statusWidth = max(minColStatus, int(float64(statusWidth)*scale))
		durationWidth = max(minColDuration, int(float64(durationWidth)*scale))
	}

	columns := []table.Column{
		{Title: "📁 Repository", Width: repoWidth},
		{Title: "🔄 Status", Width: statusWidth},
		{Title: "⏱️  Duration", Width: durationWidth},
	}

	m.Table.SetColumns(columns)
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) View() string {
	var builder strings.Builder

	// Enhanced title with gradient background
	title := titleStyle.Render("🚀 OrgSync")

	// Organization info with better styling
	orgInfo := orgStyle.Render(fmt.Sprintf("📊 Organization: %s", m.Org))

	// Progress section with stats
	completed := m.countCompleted()
	total := len(m.Repositories)
	failed := m.countFailed()

	var progressText string
	if total > 0 {
		progressText = fmt.Sprintf("Progress: %d/%d repos", completed, total)
		if failed > 0 {
			progressText += fmt.Sprintf(" (%d failed)", failed)
		}
	} else {
		progressText = "Fetching repositories..."
	}

	progressInfo := subtleText.Render(progressText)
	progressBar := progressBarStyle.Render(m.Progress.View())

	// Center content function with better spacing
	center := func(s string) string {
		return lipgloss.Place(m.Width, lipgloss.Height(s), lipgloss.Center, lipgloss.Center, s)
	}

	// Build the interface
	builder.WriteString(center(title) + "\n\n")
	builder.WriteString(center(orgInfo) + "\n\n")
	builder.WriteString(center(progressInfo) + "\n")
	builder.WriteString(center(progressBar) + "\n\n")

	if m.Done {
		summary := m.generateSummary()
		summaryBox := summaryStyle.Render(summary)
		builder.WriteString(center(summaryBox) + "\n\n")

		instruction := instructionStyle.Render("Press 'q' to quit")
		builder.WriteString(center(instruction) + "\n")
	} else {
		// Enhanced loading indicator
		loadingText := fmt.Sprintf("%s Syncing repositories...", m.Spinner.View())
		loadingSpinner := spinnerStyle.Render(loadingText)
		builder.WriteString(center(loadingSpinner) + "\n\n")

		// Table with better spacing
		if len(m.Repositories) > 0 {
			tableView := m.Table.View()
			builder.WriteString(center(tableView) + "\n\n")
		}

		instruction := instructionStyle.Render("Press 'q' or Ctrl+C to cancel")
		builder.WriteString(center(instruction) + "\n")
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

	for i, repo := range m.Repositories {
		if repo.Status == StatusCompleted {
			continue // Hide completed repos to reduce clutter
		}

		statusText := m.formatStatus(repo)
		duration := m.formatDuration(repo)

		// Truncate repository name if too long
		repoName := repo.Name
		if len(repoName) > 30 {
			repoName = repoName[:27] + "..."
		}

		row := table.Row{repoName, statusText, duration}

		// Alternate row styling
		if i%2 == 0 {
			// Even rows use default style
		} else {
			// Odd rows use alternate style - this would need custom table implementation
		}

		rows = append(rows, row)
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
			// Truncate error message to fit in column
			errMsg := repo.Error.Error()
			if len(errMsg) > 30 {
				errMsg = errMsg[:27] + "..."
			}
			errorText = fmt.Sprintf("❌ Failed: %s", errMsg)
		}
		return errorStyle.Render(errorText)
	default:
		return normalText.Render(repo.Status.String())
	}
}

func (m *Model) formatDuration(repo Repository) string {
	if repo.StartTime.IsZero() {
		return subtleText.Render("-")
	}

	end := repo.EndTime
	if end.IsZero() {
		end = time.Now()
	}

	duration := end.Sub(repo.StartTime)
	var durationText string

	if duration < time.Second {
		durationText = "<1s"
	} else if duration < time.Minute {
		durationText = fmt.Sprintf("%ds", int(duration.Seconds()))
	} else {
		durationText = fmt.Sprintf("%dm%ds", int(duration.Minutes()), int(duration.Seconds())%60)
	}

	// Color code duration based on length
	if duration > 30*time.Second {
		return errorStyle.Render(durationText)
	} else if duration > 10*time.Second {
		return pendingStyle.Render(durationText)
	}
	return subtleText.Render(durationText)
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

func (m *Model) countFailed() int {
	count := 0
	for _, repo := range m.Repositories {
		if repo.Status == StatusFailed {
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

	summary := fmt.Sprintf("🎉 Sync Complete! %d/%d successful", completed, total)
	if failed > 0 {
		summary += fmt.Sprintf(" • %d failed", failed)
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
