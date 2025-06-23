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
	Name           string
	Status         RepositoryStatus
	Error          error
	StartTime      time.Time
	EndTime        time.Time
	RetryCount     int
	Size           int64 // Size in bytes
	FilesChanged   int
	Progress       float64 // 0.0 to 1.0
	TransferSpeed  float64 // bytes per second
	LastStatusTime time.Time
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
	Org            string
	Repositories   []Repository
	Config         SyncConfig
	Done           bool
	Progress       progress.Model
	Spinner        spinner.Model
	Width          int
	Height         int
	ctx            context.Context
	cancel         context.CancelFunc
	statusChan     chan repositoryStatusMsg
	StartTime      time.Time
	TotalSize      int64
	TransferredSize int64
	SpinnerIndex   int
	AnimationTick  int
	LastUpdate     time.Time
	NetworkStatus  NetworkStatus
	ShowCompleted  bool
	// Test mode fields
	TestMode      bool
	TestRepoCount int
	TestFailRate  float64
}

const (
	padding  = 2
	minWidth = 80
	maxWidth = 140
)

type NetworkStatus int

const (
	NetworkGood NetworkStatus = iota
	NetworkSlow
	NetworkError
)

// Animation frames for different states - using ASCII for compatibility
var (
	// Professional spinner animations with consistent width
	pendingFrames = []string{"-", "\\", "|", "/"}  // Classic rotating spinner
	cloningFrames = []string{"v", "v", "v", "v"}     // Static for consistency
	fetchingFrames = []string{"~", "~", "~", "~"}    // Static for consistency
	// ASCII progress blocks
	progressBlocks = []string{" ", "=", "=", "=", "="}
)

var (
	// Enhanced gradient color palette with better contrast
	primaryGradient = []string{"#667eea", "#764ba2", "#f093fb", "#f5576c"}
	accentGradient  = []string{"#4facfe", "#00f2fe"}
	successGradient = []string{"#22c55e", "#86efac"}
	warningGradient = []string{"#f59e0b", "#fbbf24"}
	errorGradient   = []string{"#ef4444", "#f87171"}
	networkGoodGradient = []string{"#10b981", "#34d399"}
	networkSlowGradient = []string{"#f59e0b", "#fbbf24"}
	networkErrorGradient = []string{"#ef4444", "#f87171"}

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

	// Enhanced spinner with custom animation
	spn := spinner.New()
	spn.Style = spinnerStyle
	spn.Spinner = spinner.Spinner{
		Frames: fetchingFrames,
		FPS:    time.Second / 10,
	}

	return Model{
		Org:           org,
		Config:        DefaultSyncConfig(),
		Progress:      progressBar,
		Spinner:       spn,
		ctx:           ctx,
		cancel:        cancel,
		statusChan:    make(chan repositoryStatusMsg, 100),
		StartTime:     time.Now(),
		NetworkStatus: NetworkGood,
		ShowCompleted: false,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(
		m.fetchRepositories,
		m.Spinner.Tick,
		m.listenForUpdates(),
		m.animationTick(),
	)
}

// Animation tick for smooth UI updates
func (m Model) animationTick() tea.Cmd {
	return tea.Tick(time.Second/20, func(t time.Time) tea.Msg {
		return animationTickMsg{}
	})
}

// Message types
type animationTickMsg struct{}

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
		case "c":
			// Toggle showing completed repos
			m.ShowCompleted = !m.ShowCompleted
			return m, nil
		}
	case animationTickMsg:
		m.AnimationTick++
		m.LastUpdate = time.Now()
		return m, m.animationTick()
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
		
		// Update network status based on errors
		if msg.Error != nil {
			if isNetworkError(msg.Error) {
				m.NetworkStatus = NetworkSlow
			} else {
				m.NetworkStatus = NetworkError
			}
		} else if msg.Status == StatusCompleted {
			m.NetworkStatus = NetworkGood
		}

		completed := m.countCompleted()
		total := len(m.Repositories)

		if completed == total {
			m.Done = true
			// Auto-quit after a brief delay to show completion message
			return m, tea.Batch(
				m.Progress.SetPercent(1.0),
				tea.Tick(3*time.Second, func(t time.Time) tea.Msg {
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
	active := m.countActive()
	success := completed - failed

	// Title with org
	titleLine := fmt.Sprintf("%s %s %s",
		accentText.Render("OrgSync"),
		subtleText.Render("•"),
		normalText.Render(m.Org))

	// Enhanced stats line with elapsed time and ETA
	var statsLine string
	if total > 0 {
		percentage := float64(completed) / float64(total) * 100
		elapsed := time.Since(m.StartTime)
		eta := m.calculateETA()
		
		statsLine = fmt.Sprintf("%s %s %s %s %s %s %s %s %s",
			accentText.Render(fmt.Sprintf("%.0f%%", percentage)),
			subtleText.Render("•"),
			statusSuccessStyle.Render(fmt.Sprintf("%d ok", success)),
			statusErrorStyle.Render(fmt.Sprintf("%d err", failed)),
			statusActiveStyle.Render(fmt.Sprintf("%d active", active)),
			subtleText.Render("•"),
			subtleText.Render(fmt.Sprintf("%d total", total)),
			subtleText.Render("•"),
			normalText.Render(fmt.Sprintf("Time: %s / ETA: %s", formatDuration(elapsed), eta)))
	} else {
		frame := pendingFrames[m.AnimationTick%len(pendingFrames)]
		statsLine = subtleText.Render(fmt.Sprintf("%s Discovering repositories...", frame))
	}

	// Network status indicator
	networkIndicator := m.getNetworkIndicator()

	// Transfer speed if available
	transferInfo := m.getTransferInfo()

	// Enhanced progress bar
	progressBar := m.Progress.View()

	// Combine into compact header
	var headerLines []string
	headerLines = append(headerLines, titleLine)
	headerLines = append(headerLines, statsLine)
	if networkIndicator != "" {
		headerLines = append(headerLines, networkIndicator)
	}
	if transferInfo != "" {
		headerLines = append(headerLines, transferInfo)
	}
	headerLines = append(headerLines, progressBar)

	headerContent := strings.Join(headerLines, "\n")
	return cardStyle.Render(headerContent)
}

// Build compact active sync view
func (m Model) buildActiveSyncView() string {
	total := len(m.Repositories)

	if total == 0 {
		frame := pendingFrames[m.AnimationTick%len(pendingFrames)]
		return subtleText.Render(fmt.Sprintf("%s Discovering repositories...", frame))
	}

	var sections []string

	// Enhanced status indicator with current activity
	activeCount := 0
	pendingCount := 0
	failedCount := 0
	completedCount := 0

	for _, repo := range m.Repositories {
		switch repo.Status {
		case StatusCloning, StatusFetching:
			activeCount++
		case StatusPending:
			pendingCount++
		case StatusFailed:
			failedCount++
		case StatusCompleted:
			completedCount++
		}
	}

	// Build status details with animations
	var statusDetails []string
	if activeCount > 0 {
		statusDetails = append(statusDetails, statusActiveStyle.Render(fmt.Sprintf("%d active", activeCount)))
	}
	if pendingCount > 0 {
		statusDetails = append(statusDetails, statusPendingStyle.Render(fmt.Sprintf("%d queued", pendingCount)))
	}
	if completedCount > 0 {
		statusDetails = append(statusDetails, statusSuccessStyle.Render(fmt.Sprintf("%d done", completedCount)))
	}
	if failedCount > 0 {
		statusDetails = append(statusDetails, statusErrorStyle.Render(fmt.Sprintf("%d failed", failedCount)))
	}

	// Current operations info
	var currentOps []string
	for _, repo := range m.Repositories {
		if repo.Status == StatusCloning || repo.Status == StatusFetching {
			opType := "Cloning"
			if repo.Status == StatusFetching {
				opType = "Fetching"
			}
			duration := time.Since(repo.StartTime)
			currentOps = append(currentOps, fmt.Sprintf("%s %s (%s)", opType, repo.Name, formatDuration(duration)))
			if len(currentOps) >= 2 { // Show max 2 current operations
				break
			}
		}
	}

	statusLine := fmt.Sprintf("%s %s %s %s",
		m.Spinner.View(),
		accentText.Render("Syncing"),
		subtleText.Render("•"),
		strings.Join(statusDetails, " • "))
	sections = append(sections, statusLine)

	// Show current operations
	if len(currentOps) > 0 {
		opsLine := subtleText.Render("└─ " + strings.Join(currentOps, ", "))
		sections = append(sections, opsLine)
	}

	// Repository table
	tableView := m.renderCompactTable()
	if tableView != "" {
		sections = append(sections, tableView)
	}

	// Instructions
	instructions := []string{
		"'q' to cancel",
	}
	if completedCount > 0 && !m.ShowCompleted {
		instructions = append(instructions, "'c' to show completed")
	}
	instructionLine := subtleText.Render("Press " + strings.Join(instructions, " • "))
	sections = append(sections, instructionLine)

	return strings.Join(sections, "\n")
}

// Build compact completion view
func (m Model) buildCompletionView() string {
	total := len(m.Repositories)
	completed := 0
	failed := 0
	totalSize := int64(0)
	elapsed := time.Since(m.StartTime)

	for _, repo := range m.Repositories {
		switch repo.Status {
		case StatusCompleted:
			completed++
			totalSize += repo.Size
		case StatusFailed:
			failed++
		}
	}

	// Create detailed summary
	var summaryLines []string
	
	// Main celebration
	celebration := m.generateCelebration()
	summaryLines = append(summaryLines, celebration)
	summaryLines = append(summaryLines, "")

	// Detailed stats
	statsTitle := accentText.Render("📊 Summary")
	summaryLines = append(summaryLines, statsTitle)
	
	// Success/failure breakdown
	successRate := float64(completed) / float64(total) * 100
	statsLine := fmt.Sprintf("%s %d/%d (%.0f%%) • %s %d • %s %s",
		statusSuccessStyle.Render("✓ Success:"),
		completed, total, successRate,
		statusErrorStyle.Render("✗ Failed:"),
		failed,
		normalText.Render("📦 Total Size:"),
		formatBytes(totalSize))
	summaryLines = append(summaryLines, statsLine)

	// Time stats
	timeLine := fmt.Sprintf("%s %s • %s %.1f repos/min",
		normalText.Render("⏱ Duration:"),
		formatDuration(elapsed),
		normalText.Render("⚡ Speed:"),
		float64(completed)/elapsed.Minutes())
	summaryLines = append(summaryLines, timeLine)

	// Failed repos details if any
	if failed > 0 {
		summaryLines = append(summaryLines, "")
		summaryLines = append(summaryLines, statusErrorStyle.Render("❌ Failed Repositories:"))
		for _, repo := range m.Repositories {
			if repo.Status == StatusFailed {
				errorMsg := "Unknown error"
				if repo.Error != nil {
					errorMsg = m.formatErrorMessage(repo.Error)
				}
				failedLine := fmt.Sprintf("  • %s: %s", repo.Name, errorMsg)
				summaryLines = append(summaryLines, subtleText.Render(failedLine))
			}
		}
	}

	summaryLines = append(summaryLines, "")
	summaryLines = append(summaryLines, instructionStyle.Render("Exiting in 3 seconds..."))

	return strings.Join(summaryLines, "\n")
}

// Render a more compact table with animations
func (m Model) renderCompactTable() string {
	if len(m.Repositories) == 0 {
		return ""
	}

	// Count repositories by status
	activeRepos := 0
	completedRepos := 0
	for _, repo := range m.Repositories {
		if repo.Status == StatusCompleted {
			completedRepos++
		} else if repo.Status != StatusFailed || !m.ShowCompleted {
			activeRepos++
		}
	}

	if activeRepos == 0 && !m.ShowCompleted {
		return accentText.Render("🎯 All repositories processed!")
	}

	// Prioritize repos by activity level
	var displayRepos []Repository
	maxDisplay := 8

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

	// Priority 4: Show completed if toggled on
	if m.ShowCompleted {
		for _, repo := range m.Repositories {
			if repo.Status == StatusCompleted && len(displayRepos) < maxDisplay {
				displayRepos = append(displayRepos, repo)
			}
		}
	}

	if len(displayRepos) == 0 {
		return ""
	}

	// Define fixed column widths
	const (
		colRepo     = 26  // Repository name column
		colStatus   = 11  // Status column
		colTime     = 8   // Time column
		colSize     = 10  // Size column
		colProgress = 10  // Progress bar column
		colSpacer   = 1   // Space between columns
	)

	// Build enhanced table with fixed columns
	var tableLines []string

	// Header with fixed widths
	header := m.buildTableHeader(colRepo, colStatus, colTime, colSize, colProgress)
	tableLines = append(tableLines, tableHeaderStyle.Render(header))

	// Rows with animations and better info
	for i, repo := range displayRepos {
		rowStyle := tableRowStyle
		if i%2 == 1 {
			rowStyle = tableRowAltStyle
		}

		// Format each column with strict width enforcement
		repoCol := m.formatFixedWidth(m.formatRepoDisplay(repo), colRepo)
		statusCol := m.formatStatusColumn(repo, colStatus)
		timeCol := m.formatFixedWidth(m.formatDuration(repo), colTime)
		sizeCol := m.formatSizeColumn(repo, colSize)
		progressCol := m.formatProgressColumn(repo, colProgress)

		// Build row with proper spacing
		rowLine := fmt.Sprintf("%s %s %s %s %s",
			repoCol,
			statusCol,
			timeCol,
			sizeCol,
			progressCol)

		// Add visual feedback for state changes
		if repo.LastStatusTime.Add(500 * time.Millisecond).After(time.Now()) {
			// Recent status change - highlight the row
			rowStyle = rowStyle.Background(lipgloss.Color("#4A5568"))
		}

		tableLines = append(tableLines, rowStyle.Render(rowLine))
	}

	result := strings.Join(tableLines, "\n")

	// Show remaining count and toggle hint
	var footer []string
	if (activeRepos + completedRepos) > maxDisplay {
		remaining := (activeRepos + completedRepos) - maxDisplay
		footer = append(footer, subtleText.Render(fmt.Sprintf("... and %d more repositories", remaining)))
	}
	if completedRepos > 0 && !m.ShowCompleted {
		footer = append(footer, subtleText.Render("Press 'c' to show completed"))
	} else if m.ShowCompleted {
		footer = append(footer, subtleText.Render("Press 'c' to hide completed"))
	}

	if len(footer) > 0 {
		result += "\n" + strings.Join(footer, " • ")
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

	// Animated celebration with different messages based on success rate
	successRate := float64(completed) / float64(total)
	
	if failed == 0 {
		celebrations := []string{
			"🎉 Perfect! All %d repositories synchronized successfully! 🎉",
			"🌟 Flawless! %d repositories synced without errors! 🌟",
			"🏆 Champion! %d repositories completed perfectly! 🏆",
			"✨ Excellent! All %d repositories are up to date! ✨",
		}
		return fmt.Sprintf(celebrations[m.AnimationTick%len(celebrations)], completed)
	} else if successRate >= 0.8 {
		return fmt.Sprintf("🎯 Great job! %d/%d successful (%.0f%%) • %d need attention",
			completed, total, successRate*100, failed)
	} else if successRate >= 0.5 {
		return fmt.Sprintf("✅ Sync Complete. %d/%d successful (%.0f%%) • %d failed",
			completed, total, successRate*100, failed)
	} else {
		return fmt.Sprintf("⚠️ Sync finished with issues. %d/%d successful • %d failed",
			completed, total, failed)
	}
}

// Message types
type repositoriesFetchedMsg struct {
	Repositories []Repository
}

type repositoryStatusMsg struct {
	Name          string
	Status        RepositoryStatus
	Error         error
	Size          int64
	FilesChanged  int
	TransferSpeed float64
}

// Helper methods
func (m *Model) updateRepositoryStatus(name string, status RepositoryStatus, err error) {
	for i := range m.Repositories {
		if m.Repositories[i].Name == name {
			m.Repositories[i].Status = status
			m.Repositories[i].Error = err
			m.Repositories[i].LastStatusTime = time.Now()

			if status == StatusCloning || status == StatusFetching {
				if m.Repositories[i].StartTime.IsZero() {
					m.Repositories[i].StartTime = time.Now()
				}
			} else if status == StatusCompleted || status == StatusFailed {
				m.Repositories[i].EndTime = time.Now()
				if status == StatusFailed && err != nil {
					m.Repositories[i].RetryCount++
				}
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

func (m *Model) countActive() int {
	count := 0
	for _, repo := range m.Repositories {
		if repo.Status == StatusCloning || repo.Status == StatusFetching {
			count++
		}
	}
	return count
}

// Animation helpers
func (m *Model) getActiveAnimation() string {
	// Return empty string - no animation needed for active count
	return ""
}

func (m *Model) getNetworkIndicator() string {
	switch m.NetworkStatus {
	case NetworkGood:
		return statusSuccessStyle.Render("Network: Excellent")
	case NetworkSlow:
		return statusPendingStyle.Render("Network: Slow")
	case NetworkError:
		return statusErrorStyle.Render("Network: Issues")
	default:
		return ""
	}
}

func (m *Model) getTransferInfo() string {
	totalSpeed := float64(0)
	activeTransfers := 0
	for _, repo := range m.Repositories {
		if repo.Status == StatusCloning || repo.Status == StatusFetching {
			totalSpeed += repo.TransferSpeed
			activeTransfers++
		}
	}
	
	if activeTransfers == 0 || totalSpeed == 0 {
		return ""
	}
	
	return subtleText.Render(fmt.Sprintf("%s/s", formatBytes(int64(totalSpeed))))
}

func (m *Model) calculateETA() string {
	pending := 0
	for _, repo := range m.Repositories {
		if repo.Status == StatusPending {
			pending++
		}
	}
	
	if pending == 0 {
		return "soon"
	}
	
	// Estimate based on average completion time
	avgTime := m.getAverageCompletionTime()
	if avgTime == 0 {
		return "calculating..."
	}
	
	estimatedSeconds := float64(pending) * avgTime.Seconds() / float64(m.Config.MaxConcurrency)
	return formatDuration(time.Duration(estimatedSeconds) * time.Second)
}

func (m *Model) getAverageCompletionTime() time.Duration {
	totalTime := time.Duration(0)
	completedCount := 0
	
	for _, repo := range m.Repositories {
		if repo.Status == StatusCompleted && !repo.StartTime.IsZero() && !repo.EndTime.IsZero() {
			totalTime += repo.EndTime.Sub(repo.StartTime)
			completedCount++
		}
	}
	
	if completedCount == 0 {
		return 0
	}
	
	return totalTime / time.Duration(completedCount)
}

// Build table header with fixed column widths
func (m *Model) buildTableHeader(colRepo, colStatus, colTime, colSize, colProgress int) string {
	repoHeader := m.padRight("Repository", colRepo)
	statusHeader := m.padRight("Status", colStatus)
	timeHeader := m.padRight("Time", colTime)
	sizeHeader := m.padRight("Size", colSize)
	progressHeader := m.padRight("Progress", colProgress)
	
	return fmt.Sprintf("%s %s %s %s %s",
		repoHeader,
		statusHeader,
		timeHeader,
		sizeHeader,
		progressHeader)
}

// Format string to fixed width with proper truncation and left alignment
func (m *Model) formatFixedWidth(s string, width int) string {
	// Strip any ANSI codes for accurate length calculation
	plainText := stripAnsi(s)
	runes := []rune(plainText)
	visualLen := len(runes)
	
	if visualLen > width {
		// Truncate with ellipsis, preserving left alignment
		return string(runes[:width-1]) + "…"
	} else if visualLen < width {
		// Left-align by padding with spaces on the right
		return s + strings.Repeat(" ", width-visualLen)
	}
	return s
}

// Helper to pad string to the right (left-align text)
func (m *Model) padRight(s string, width int) string {
	runes := []rune(s)
	if len(runes) >= width {
		return string(runes[:width])
	}
	// Left-align by padding spaces on the right
	return s + strings.Repeat(" ", width-len(runes))
}

// Strip ANSI codes from string for length calculation
func stripAnsi(s string) string {
	// Remove all ANSI escape sequences
	var result strings.Builder
	var inEscape bool
	
	for _, r := range s {
		if r == '\033' {
			inEscape = true
			continue
		}
		
		if inEscape {
			if r == 'm' {
				inEscape = false
			}
			continue
		}
		
		result.WriteRune(r)
	}
	
	return result.String()
}

// Formatting helpers
func (m *Model) formatRepoDisplay(repo Repository) string {
	icon := m.getRepoIcon(repo)
	name := repo.Name
	
	// Calculate space: icon (2) + space (1) = 3 chars used
	// Total column width is 26, so 23 chars available for name
	availableSpace := 23
	
	if repo.RetryCount > 0 && repo.Status == StatusFailed {
		// Need space for " (2✕)" which is 5 chars
		availableSpace -= 5
	}
	
	// Truncate name if needed using rune count for proper Unicode handling
	nameRunes := []rune(name)
	if len(nameRunes) > availableSpace {
		name = string(nameRunes[:availableSpace-1]) + "…"
	}
	
	// Build left-aligned display string
	if repo.RetryCount > 0 && repo.Status == StatusFailed {
		return fmt.Sprintf("%s %s (%dx)", icon, name, repo.RetryCount)
	}
	
	return fmt.Sprintf("%s %s", icon, name)
}

func (m *Model) getRepoIcon(repo Repository) string {
	switch repo.Status {
	case StatusPending:
		// Subtle rotating spinner - slow rotation for professional look
		frame := pendingFrames[(m.AnimationTick/4)%len(pendingFrames)]
		return frame + " "
	case StatusCloning:
		// Static download arrow
		return "v "
	case StatusFetching:
		// Static sync symbol
		return "~ "
	case StatusCompleted:
		return "o "
	case StatusFailed:
		return "x "
	default:
		return "- "
	}
}

func (m *Model) formatStatusDisplay(repo Repository) string {
	// Return plain text - styling will be applied after width formatting
	switch repo.Status {
	case StatusPending:
		return "Queued"
	case StatusCloning:
		return "Cloning"
	case StatusFetching:
		return "Fetching"
	case StatusCompleted:
		return "Done"
	case StatusFailed:
		if repo.Error != nil {
			// Show brief error type
			if strings.Contains(repo.Error.Error(), "authentication") {
				return "Auth Err"
			} else if strings.Contains(repo.Error.Error(), "not found") {
				return "Not Found"
			} else if strings.Contains(repo.Error.Error(), "timeout") {
				return "Timeout"
			}
		}
		return "Failed"
	default:
		return "Unknown"
	}
}

func (m *Model) formatSize(repo Repository) string {
	if repo.Size == 0 {
		return "-"
	}
	return formatBytes(repo.Size)
}

func (m *Model) formatProgress(repo Repository) string {
	// Fixed width progress bars (10 chars to match column)
	switch repo.Status {
	case StatusCloning, StatusFetching:
		// Animated progress bar
		return m.renderMiniProgressBar(repo.Progress)
	case StatusCompleted:
		return "[========]"
	case StatusFailed:
		return "[FAILED  ]"
	case StatusPending:
		// Clean static waiting indicator
		return "[  wait  ]"
	default:
		return "[        ]" // 10 chars total
	}
}

// Format status column with styling applied after width formatting
func (m *Model) formatStatusColumn(repo Repository, width int) string {
	status := m.formatStatusDisplay(repo)
	padded := m.padRight(status, width)
	
	// Apply styling based on status
	switch repo.Status {
	case StatusPending:
		return statusPendingStyle.Render(padded)
	case StatusCloning, StatusFetching:
		return statusActiveStyle.Render(padded)
	case StatusCompleted:
		return statusSuccessStyle.Render(padded)
	case StatusFailed:
		return statusErrorStyle.Render(padded)
	default:
		return normalText.Render(padded)
	}
}

// Format size column with styling
func (m *Model) formatSizeColumn(repo Repository, width int) string {
	size := m.formatSize(repo)
	padded := m.padRight(size, width)
	
	if repo.Size == 0 {
		return subtleText.Render(padded)
	}
	return normalText.Render(padded)
}

// Format progress column with styling
func (m *Model) formatProgressColumn(repo Repository, width int) string {
	progress := m.formatProgress(repo)
	// Progress is already fixed width (10 chars), ensure it fits the column
	if width != 10 {
		progress = m.padRight(progress, width)
	}
	
	// Apply styling based on status
	switch repo.Status {
	case StatusCloning, StatusFetching:
		// Pulsing animation for active transfers
		if m.AnimationTick%10 < 5 {
			return statusActiveStyle.Render(progress)
		}
		return accentText.Render(progress)
	case StatusCompleted:
		return statusSuccessStyle.Render(progress)
	case StatusFailed:
		return statusErrorStyle.Render(progress)
	case StatusPending:
		return subtleText.Render(progress)
	default:
		return progress
	}
}

func (m *Model) renderMiniProgressBar(progress float64) string {
	// Create clean, professional progress bar [====    ] total 10 chars
	const innerWidth = 8 // Space inside brackets
	filled := int(progress * float64(innerWidth))
	
	if filled > innerWidth {
		filled = innerWidth
	}
	
	// Build the progress bar
	bar := "["
	
	// Add filled portion with solid blocks
	for i := 0; i < filled; i++ {
		bar += "="
	}
	
	// Fill remainder with spaces for clean look
	for i := filled; i < innerWidth; i++ {
		bar += " "
	}
	
	bar += "]"
	
	return bar // Exactly 10 characters
}

func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return "<1s"
	} else if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	} else if d < time.Hour {
		return fmt.Sprintf("%dm%ds", int(d.Minutes()), int(d.Seconds())%60)
	} else {
		return fmt.Sprintf("%dh%dm", int(d.Hours()), int(d.Minutes())%60)
	}
}

// Business logic
func (m Model) fetchRepositories() tea.Msg {
	var repos []string
	var err error

	if m.TestMode {
		// Generate test repositories
		repos = m.generateTestRepos()
	} else {
		repos, err = fetchReposInOrg(m.ctx, m.Org)
		if err != nil {
			return repositoriesFetchedMsg{
				Repositories: []Repository{{Name: "Error fetching repos", Status: StatusFailed, Error: err}},
			}
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

				// Determine initial status
				initialStatus := StatusCloning
				if m.TestMode {
					// In test mode, randomly decide if it's a clone or fetch
					if randFloat() > 0.6 {
						initialStatus = StatusFetching
					}
				}
				
				// Send initial status
				select {
				case m.statusChan <- repositoryStatusMsg{
					Name:   r.Name,
					Status: initialStatus,
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

				// Send final status with additional info
				select {
				case m.statusChan <- repositoryStatusMsg{
					Name:   r.Name,
					Status: status,
					Error:  err,
					Size:   0, // Could be enhanced to get actual repo size
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
	if m.TestMode {
		return m.simulateRepoSync(repoName)
	}

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

func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	
	errStr := err.Error()
	return strings.Contains(errStr, "timeout") ||
		strings.Contains(errStr, "connection") ||
		strings.Contains(errStr, "network") ||
		strings.Contains(errStr, "dial tcp")
}

func (m *Model) formatErrorMessage(err error) string {
	if err == nil {
		return "Unknown error"
	}
	
	errStr := err.Error()
	
	// Categorize and simplify error messages
	if strings.Contains(errStr, "authentication") {
		return "Authentication failed - check 'gh auth status'"
	} else if strings.Contains(errStr, "permission denied") {
		return "Permission denied - check repository access"
	} else if strings.Contains(errStr, "not found") || strings.Contains(errStr, "repository not found") {
		return "Repository not found or access denied"
	} else if strings.Contains(errStr, "timeout") {
		return "Operation timed out - network may be slow"
	} else if strings.Contains(errStr, "connection refused") {
		return "Connection refused - check network settings"
	} else if strings.Contains(errStr, "no such host") {
		return "DNS resolution failed - check internet connection"
	}
	
	// Truncate long errors
	if len(errStr) > 50 {
		return errStr[:47] + "..."
	}
	
	return errStr
}

// Test mode implementations

// Generate test repository names
func (m Model) generateTestRepos() []string {
	var repos []string
	projects := []string{"api", "web", "mobile", "backend", "frontend", "service", "lib", "tool", "sdk", "cli", "app", "core", "data", "auth", "utils", "gateway", "worker", "admin", "dashboard", "analytics"}
	languages := []string{"go", "js", "py", "java", "rust", "swift", "kotlin", "rb", "cpp", "ts", "cs", "php", "scala", "elixir", "dart"}

	for i := 0; i < m.TestRepoCount; i++ {
		project := projects[i%len(projects)]
		lang := languages[(i/2)%len(languages)]
		repos = append(repos, fmt.Sprintf("%s-%s-%d", project, lang, i+1))
	}
	return repos
}

// Simulate repository sync operation
func (m Model) simulateRepoSync(repoName string) error {
	// Simulate random operation duration
	baseTime := 2 * time.Second
	variability := 3 * time.Second
	duration := baseTime + time.Duration(float64(variability)*randFloat())
	
	// Simulate progress updates
	progressTicker := time.NewTicker(100 * time.Millisecond)
	defer progressTicker.Stop()
	
	startTime := time.Now()
	endTime := startTime.Add(duration)
	
	// Send progress updates
	for {
		select {
		case <-progressTicker.C:
			elapsed := time.Since(startTime)
			progress := float64(elapsed) / float64(duration)
			if progress > 1.0 {
				progress = 1.0
			}
			
			// Calculate simulated transfer speed
			transferSpeed := float64(1024*1024) * (1 + randFloat()*2) // 1-3 MB/s
			size := int64(float64(5*1024*1024) * (1 + randFloat()*10)) // 5-50 MB
			
			// Update repository progress
			for i := range m.Repositories {
				if m.Repositories[i].Name == repoName {
					m.Repositories[i].Progress = progress
					m.Repositories[i].TransferSpeed = transferSpeed
					m.Repositories[i].Size = size
					break
				}
			}
			
			if time.Now().After(endTime) {
				break
			}
			
		case <-m.ctx.Done():
			return m.ctx.Err()
		}
	}
	
	// Simulate random failures based on test fail rate
	if randFloat() < m.TestFailRate {
		return m.generateTestError(repoName)
	}
	
	return nil
}

// Generate realistic test errors
func (m Model) generateTestError(repoName string) error {
	errors := []string{
		"authentication required",
		"repository not found",
		"permission denied",
		"connection timeout",
		"network unreachable",
		"operation timed out",
		"SSL certificate problem",
		"rate limit exceeded",
	}
	
	errorIndex := int(randFloat() * float64(len(errors)))
	if errorIndex >= len(errors) {
		errorIndex = len(errors) - 1
	}
	
	return fmt.Errorf("%s: %s", errors[errorIndex], repoName)
}

// Simple random float generator (0.0 to 1.0)
func randFloat() float64 {
	// Use current time nanoseconds for pseudo-randomness
	return float64(time.Now().UnixNano()%1000) / 1000.0
}
