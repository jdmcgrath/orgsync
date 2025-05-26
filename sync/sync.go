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
	Width        int
	Height       int
	ctx          context.Context
	cancel       context.CancelFunc
	statusChan   chan repositoryStatusMsg
}

const (
	padding  = 2
	minWidth = 80
	maxWidth = 140
)

var (
	// Modern gradient color palette
	primaryGradient = []string{"#667eea", "#764ba2", "#f093fb", "#f5576c"}
	accentGradient  = []string{"#4facfe", "#00f2fe"}
	successGradient = []string{"#11998e", "#38ef7d"}
	warningGradient = []string{"#f093fb", "#f5576c"}
	errorGradient   = []string{"#fc466b", "#3f5efb"}

	// Hero title with stunning gradient and glow effect
	heroTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("#FFFFFF")).
			Background(lipgloss.AdaptiveColor{Light: "#667eea", Dark: "#667eea"}).
			Padding(1, 4).
			MarginBottom(1).
			Border(lipgloss.ThickBorder()).
			BorderForeground(lipgloss.AdaptiveColor{Light: "#f093fb", Dark: "#f093fb"}).
			Align(lipgloss.Center)

	// Elegant organization card
	orgCardStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")).
			Background(lipgloss.AdaptiveColor{Light: "#2D3748", Dark: "#1A202C"}).
			Padding(1, 3).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4A5568")).
			MarginBottom(1).
			Align(lipgloss.Center)

	// Stats panel with modern design
	statsStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0AEC0")).
			Background(lipgloss.AdaptiveColor{Light: "#2D3748", Dark: "#1A202C"}).
			Padding(0, 2).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4A5568")).
			Align(lipgloss.Center)

	// Enhanced progress container
	progressContainerStyle = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("#4A5568")).
				Padding(1, 2).
				Background(lipgloss.AdaptiveColor{Light: "#2D3748", Dark: "#1A202C"})

	// Animated spinner with glow
	spinnerStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#f093fb")).
			Bold(true).
			Blink(false)

	// Status indicators with modern styling
	statusPendingStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#F6AD55")).
				Bold(true)

	statusActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#4facfe")).
				Bold(true)

	statusSuccessStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#38ef7d")).
				Bold(true)

	statusErrorStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#fc466b")).
				Bold(true)

	// Modern table styling
	tableHeaderStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.Color("#4A5568")).
				Padding(0, 2).
				Border(lipgloss.NormalBorder(), false, false, true, false).
				BorderForeground(lipgloss.Color("#667eea")).
				Align(lipgloss.Center)

	tableRowStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0")).
			Background(lipgloss.AdaptiveColor{Light: "#2D3748", Dark: "#1A202C"}).
			Padding(0, 1)

	tableRowAltStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#E2E8F0")).
				Background(lipgloss.AdaptiveColor{Light: "#374151", Dark: "#2D3748"}).
				Padding(0, 1)

	// Completion celebration
	celebrationStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("#FFFFFF")).
				Background(lipgloss.AdaptiveColor{Light: "#38ef7d", Dark: "#11998e"}).
				Padding(2, 4).
				Border(lipgloss.ThickBorder()).
				BorderForeground(lipgloss.Color("#38ef7d")).
				MarginTop(1).
				Align(lipgloss.Center)

	// Subtle instruction text
	instructionStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("#718096")).
				Italic(true).
				Align(lipgloss.Center)

	// Text styles
	normalText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#E2E8F0"))

	subtleText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#A0AEC0"))

	accentText = lipgloss.NewStyle().
			Foreground(lipgloss.Color("#4facfe")).
			Bold(true)

	// Card container for sections
	cardStyle = lipgloss.NewStyle().
			Background(lipgloss.AdaptiveColor{Light: "#2D3748", Dark: "#1A202C"}).
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("#4A5568")).
			Padding(1, 2).
			MarginBottom(1)
)

func NewModel(org string) Model {
	ctx, cancel := context.WithCancel(context.Background())

	// Stunning progress bar with modern gradient
	progressBar := progress.New(
		progress.WithScaledGradient("#4facfe", "#00f2fe"),
		progress.WithoutPercentage(),
		progress.WithWidth(60),
	)

	// Enhanced spinner
	spn := spinner.New()
	spn.Style = spinnerStyle
	spn.Spinner = spinner.Dot

	// Table is no longer used - we render manually for better control

	return Model{
		Org:        org,
		Config:     DefaultSyncConfig(),
		Progress:   progressBar,
		Spinner:    spn,
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
		return m, m.syncRepositories()
	case repositoryStatusMsg:
		m.updateRepositoryStatus(msg.Name, msg.Status, msg.Error)

		completed := m.countCompleted()
		total := len(m.Repositories)

		if completed == total {
			m.Done = true
			// Auto-quit after a brief delay to show completion message
			return m, tea.Batch(
				m.Progress.SetPercent(1.0),
				tea.Tick(2*time.Second, func(t time.Time) tea.Msg {
					return tea.Quit()
				}),
			)
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

// updateTableColumns is no longer needed as we use manual table rendering

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func (m Model) View() string {
	// Build the complete UI
	var sections []string

	// Header section
	header := m.buildCompactHeader()
	sections = append(sections, header)

	if m.Done {
		// Completion view
		completion := m.buildCompletionView()
		sections = append(sections, completion)
	} else {
		// Active sync view
		activeView := m.buildActiveSyncView()
		if activeView != "" {
			sections = append(sections, activeView)
		}
	}

	// Join sections with proper spacing
	content := strings.Join(sections, "\n\n")

	// Center the content vertically and horizontally
	return m.centerContent(content)
}

// Build a compact, information-dense header
func (m Model) buildCompactHeader() string {
	completed := m.countCompleted()
	total := len(m.Repositories)
	failed := m.countFailed()
	active := total - completed

	// Compact title with org and stats in one line
	titleLine := fmt.Sprintf("🚀 %s %s %s",
		accentText.Render("OrgSync"),
		subtleText.Render("•"),
		normalText.Render(m.Org))

	// Compact stats line
	var statsLine string
	if total > 0 {
		percentage := float64(completed) / float64(total) * 100
		statsLine = fmt.Sprintf("%s %s %s %s %s %s %s",
			accentText.Render(fmt.Sprintf("%.0f%%", percentage)),
			subtleText.Render("•"),
			statusSuccessStyle.Render(fmt.Sprintf("%d✓", completed-failed)),
			statusErrorStyle.Render(fmt.Sprintf("%d✗", failed)),
			statusActiveStyle.Render(fmt.Sprintf("%d⚡", active)),
			subtleText.Render("•"),
			subtleText.Render(fmt.Sprintf("%d total", total)))
	} else {
		statsLine = subtleText.Render("🔍 Discovering repositories...")
	}

	// Progress bar (more compact)
	progressBar := m.Progress.View()

	// Combine into compact header with better spacing
	var headerLines []string
	headerLines = append(headerLines, titleLine)
	headerLines = append(headerLines, statsLine)
	headerLines = append(headerLines, progressBar)

	headerContent := strings.Join(headerLines, "\n")
	return cardStyle.Render(headerContent)
}

// Build compact active sync view
func (m Model) buildActiveSyncView() string {
	total := len(m.Repositories)

	if total == 0 {
		return subtleText.Render("🔍 Discovering repositories...")
	}

	var sections []string

	// Enhanced status indicator with current activity
	activeCount := 0
	pendingCount := 0
	failedCount := 0

	for _, repo := range m.Repositories {
		switch repo.Status {
		case StatusCloning, StatusFetching:
			activeCount++
		case StatusPending:
			pendingCount++
		case StatusFailed:
			failedCount++
		}
	}

	var statusDetails []string
	if activeCount > 0 {
		statusDetails = append(statusDetails, statusActiveStyle.Render(fmt.Sprintf("%d active", activeCount)))
	}
	if pendingCount > 0 {
		statusDetails = append(statusDetails, statusPendingStyle.Render(fmt.Sprintf("%d queued", pendingCount)))
	}
	if failedCount > 0 {
		statusDetails = append(statusDetails, statusErrorStyle.Render(fmt.Sprintf("%d failed", failedCount)))
	}

	statusLine := fmt.Sprintf("%s %s %s %s",
		m.Spinner.View(),
		accentText.Render("Syncing"),
		subtleText.Render("•"),
		strings.Join(statusDetails, " • "))
	sections = append(sections, statusLine)

	// Compact table
	tableView := m.renderCompactTable()
	if tableView != "" {
		sections = append(sections, tableView)
	}

	// Compact instructions
	instruction := subtleText.Render("Press 'q' to cancel")
	sections = append(sections, instruction)

	return strings.Join(sections, "\n")
}

// Build compact completion view
func (m Model) buildCompletionView() string {
	celebration := m.generateCelebration()
	instruction := subtleText.Render("Exiting in 2 seconds...")

	var completionLines []string
	completionLines = append(completionLines, celebration)
	completionLines = append(completionLines, instruction)

	completionContent := strings.Join(completionLines, "\n")
	return celebrationStyle.Render(completionContent)
}

// Render a more compact table
func (m Model) renderCompactTable() string {
	if len(m.Repositories) == 0 {
		return ""
	}

	// Count active repositories
	activeRepos := 0
	for _, repo := range m.Repositories {
		if repo.Status != StatusCompleted {
			activeRepos++
		}
	}

	if activeRepos == 0 {
		return accentText.Render("🎯 All repositories processed!")
	}

	// Prioritize repos by activity level - show most interesting first
	var displayRepos []Repository
	maxDisplay := 6 // Reduced for better fit

	// Priority 1: Currently active (cloning/fetching)
	for _, repo := range m.Repositories {
		if (repo.Status == StatusCloning || repo.Status == StatusFetching) && len(displayRepos) < maxDisplay {
			displayRepos = append(displayRepos, repo)
		}
	}

	// Priority 2: Failed repos (need attention)
	for _, repo := range m.Repositories {
		if repo.Status == StatusFailed && len(displayRepos) < maxDisplay {
			displayRepos = append(displayRepos, repo)
		}
	}

	// Priority 3: Pending repos (waiting to start)
	for _, repo := range m.Repositories {
		if repo.Status == StatusPending && len(displayRepos) < maxDisplay {
			displayRepos = append(displayRepos, repo)
		}
	}

	if len(displayRepos) == 0 {
		return ""
	}

	// Build table manually with perfect alignment
	var tableLines []string

	// Header
	headerLine := fmt.Sprintf("%-28s %-12s %-8s %s", "Repository", "Status", "Time", "Progress")
	tableLines = append(tableLines, tableHeaderStyle.Render(headerLine))

	// Rows
	for _, repo := range displayRepos {
		statusText := m.formatStatusSimple(repo)
		duration := m.formatDuration(repo)

		// Truncate repo name to fit column width
		repoName := repo.Name
		maxRepoLen := 20
		if len(repoName) > maxRepoLen {
			repoName = repoName[:maxRepoLen-1] + "…"
		}

		// Add status icon
		var icon string
		switch repo.Status {
		case StatusPending:
			icon = "📦"
		case StatusCloning:
			icon = "📥"
		case StatusFetching:
			icon = "🔄"
		case StatusFailed:
			icon = "💥"
		default:
			icon = "📁"
		}

		// Simple progress indicator
		var progressIndicator string
		if repo.Status == StatusCloning || repo.Status == StatusFetching {
			progressIndicator = "●●●"
		} else if repo.Status == StatusFailed {
			progressIndicator = "✗"
		} else if repo.Status == StatusPending {
			progressIndicator = "○○○"
		} else {
			progressIndicator = ""
		}

		repoColumn := fmt.Sprintf("%s %s", icon, repoName)
		rowLine := fmt.Sprintf("%-28s %-12s %-8s %s", repoColumn, statusText, duration, progressIndicator)
		tableLines = append(tableLines, tableRowStyle.Render(rowLine))
	}

	result := strings.Join(tableLines, "\n")

	// Show remaining count if there are more
	if activeRepos > maxDisplay {
		remaining := activeRepos - maxDisplay
		remainingLine := subtleText.Render(fmt.Sprintf("... and %d more repositories", remaining))
		result += "\n" + remainingLine
	}

	return result
}

// Format status without color styling for better alignment
func (m *Model) formatStatusSimple(repo Repository) string {
	switch repo.Status {
	case StatusPending:
		return "Pending"
	case StatusCloning:
		return "Cloning"
	case StatusFetching:
		return "Fetching"
	case StatusCompleted:
		return "Completed"
	case StatusFailed:
		return "Failed"
	default:
		return "Unknown"
	}
}

// Helper function to center content
func (m Model) centerContent(content string) string {
	// Add some top padding for vertical centering
	topPadding := "\n\n\n"

	// Use lipgloss to center each line horizontally
	if m.Width > 0 {
		centeredContent := lipgloss.NewStyle().
			Width(m.Width).
			Align(lipgloss.Center).
			Render(content)
		return topPadding + centeredContent
	}

	// Fallback: just add top padding
	return topPadding + content
}

// Generate celebration message
func (m *Model) generateCelebration() string {
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

	if failed == 0 {
		return fmt.Sprintf("🎉 Perfect! All %d repositories synchronized successfully! 🎉", completed)
	} else {
		return fmt.Sprintf("✅ Sync Complete! %d/%d successful • %d failed", completed, total, failed)
	}
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

// updateTable is no longer needed as we use manual table rendering

func (m *Model) formatStatus(repo Repository) string {
	switch repo.Status {
	case StatusPending:
		return statusPendingStyle.Render("Pending")
	case StatusCloning:
		return statusActiveStyle.Render("Cloning")
	case StatusFetching:
		return statusActiveStyle.Render("Fetching")
	case StatusCompleted:
		return statusSuccessStyle.Render("Completed")
	case StatusFailed:
		return statusErrorStyle.Render("Failed")
	default:
		return normalText.Render("Unknown")
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
	var durationText string

	if duration < time.Second {
		durationText = "<1s"
	} else if duration < time.Minute {
		durationText = fmt.Sprintf("%ds", int(duration.Seconds()))
	} else {
		durationText = fmt.Sprintf("%dm%ds", int(duration.Minutes()), int(duration.Seconds())%60)
	}

	return durationText
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
