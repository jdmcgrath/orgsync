package sync

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/table"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type Repository struct {
	Name string
	Done bool
	Err  error
}

type Model struct {
	Org          string
	Repositories []Repository
	Done         bool
	Errors       []error
	Progress     progress.Model
	Spinner      spinner.Model
	Table        table.Model
	Width        int
	Height       int
}

const (
	padding  = 2
	maxWidth = 80
)

var (
	titleStyle   = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#FFDD00")).Background(lipgloss.Color("#336699"))
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")) // Orange
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")) // Red
	spinnerStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
	normalText   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFFFFF"))
)

func NewModel(org string) Model {
	progressBar := progress.New(progress.WithDefaultGradient(), progress.WithScaledGradient("#FFA500", "#00FF00"))
	spn := spinner.New()
	spn.Style = spinnerStyle

	columns := []table.Column{
		{Title: "Repository", Width: 30},
		{Title: "Status", Width: 30},
	}

	tbl := table.New(
		table.WithColumns(columns),
		table.WithHeight(10),
	)

	return Model{
		Org:      org,
		Progress: progressBar,
		Spinner:  spn,
		Table:    tbl,
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(m.fetchRepositories, m.Spinner.Tick)
}

// Update processes messages and updates the state of the Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" {
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
		rows := make([]table.Row, len(m.Repositories))
		for i, repo := range m.Repositories {
			rows[i] = table.Row{repo.Name, pendingStyle.Render("Pending")}
		}
		m.Table.SetRows(rows)
		return m, tea.Batch(m.syncRepositories()...)
	case repositoryProcessedMsg:
		// Update repository details in the model
		for i := range m.Repositories {
			if m.Repositories[i].Name == msg.Repo.Name {
				m.Repositories[i].Done = true
				m.Repositories[i].Err = msg.Err
				break
			}
		}

		// Update the table
		rows := m.Table.Rows()
		for i, row := range rows {
			if row[0] == msg.Repo.Name {
				if msg.Err != nil {
					rows[i][1] = errorStyle.Render(fmt.Sprintf("Error: %v", msg.Err))
				}
				break
			}
		}
		m.Table.SetRows(rows)

		// Remove completed repositories from the table
		if msg.Err == nil {
			m.Table.SetRows(removeRow(m.Table.Rows(), msg.Repo.Name))
		}

		// Calculate the number of completed repositories
		completed := 0
		for _, repo := range m.Repositories {
			if repo.Done {
				completed++
			}
		}

		// Determine if all repositories are done and quit if true
		if m.Done = completed == len(m.Repositories); m.Done {
			return m, tea.Batch(m.Progress.SetPercent(100))
		}
		return m, m.Progress.SetPercent(float64(completed) / float64(len(m.Repositories)))

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.Spinner, cmd = m.Spinner.Update(msg)
		return m, cmd
	case progress.FrameMsg:
		// Handle progress bar animation
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
	loadingSpinner := m.Spinner.View() + " Loading..."
	tableView := m.Table.View()

	center := func(s string) string {
		return lipgloss.Place(m.Width, len(strings.Split(s, "\n")), lipgloss.Center, lipgloss.Center, s)
	}

	builder.WriteString(center(title) + "\n\n")
	builder.WriteString(center(orgInfo) + "\n\n")
	builder.WriteString(center(progressBar) + "\n\n")

	if m.Done {
		builder.WriteString(center("All operations completed. Press 'q' to quit.") + "\n")
	} else {
		builder.WriteString(center(loadingSpinner) + "\n\n")
		builder.WriteString(center(tableView) + "\n")
		builder.WriteString(center("Press 'q' to quit.") + "\n")
	}

	return builder.String()
}

// repositoriesFetchedMsg contains the fetched repositories
type repositoriesFetchedMsg struct {
	Repositories []Repository
}

// repositoryProcessedMsg contains the processed repository status
type repositoryProcessedMsg struct {
	Repo Repository
	Err  error
}

// fetchRepositories retrieves repositories and returns a message containing the result
func (m Model) fetchRepositories() tea.Msg {
	repos, err := fetchReposInOrg(m.Org)
	if err != nil {
		return repositoriesFetchedMsg{Repositories: []Repository{{Name: "Error fetching repos"}}}
	}
	repositories := make([]Repository, len(repos))
	for i, repo := range repos {
		repositories[i] = Repository{Name: repo}
	}
	return repositoriesFetchedMsg{Repositories: repositories}
}

// syncRepositories triggers commands to clone or fetch each repository
func (m Model) syncRepositories() []tea.Cmd {
	cmds := make([]tea.Cmd, len(m.Repositories))
	for i, repo := range m.Repositories {
		cmds[i] = syncRepositoryCmd(m.Org, repo)
	}
	return cmds
}

func syncRepositoryCmd(org string, repo Repository) tea.Cmd {
	return func() tea.Msg {
		time.Sleep(1 * time.Second) // simulate some delay
		err := syncRepo(org, repo.Name)
		return repositoryProcessedMsg{Repo: repo, Err: err}
	}
}

func fetchReposInOrg(org string) ([]string, error) {
	cmd := exec.Command("gh", "repo", "list", org, "--json", "name", "--jq", ".[] | .name", "--limit", "1000")
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("failed to fetch repos: %w", err)
	}

	repos := strings.Split(strings.TrimSpace(out.String()), "\n")
	return repos, nil
}

func repoExists(repoDir string) bool {
	_, err := os.Stat(repoDir)
	return !os.IsNotExist(err)
}

func cloneRepo(org, repo, repoDir string) error {
	cmd := exec.Command("gh", "repo", "clone", fmt.Sprintf("%s/%s", org, repo), repoDir)

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone %s: %w", repo, err)
	}
	return nil
}

func fetchRepo(repoDir, repo string) error {
	cmd := exec.Command("git", "-C", repoDir, "fetch", "origin")

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch %s: %w", repo, err)
	}
	return nil
}

func syncRepo(org, repo string) error {
	repoDir := filepath.Join(".", repo)

	if repoExists(repoDir) {
		return fetchRepo(repoDir, repo)
	} else {
		return cloneRepo(org, repo, repoDir)
	}
}

func removeRow(rows []table.Row, repoName string) []table.Row {
	for i, row := range rows {
		if row[0] == repoName {
			return append(rows[:i], rows[i+1:]...)
		}
	}
	return rows
}
