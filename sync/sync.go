package sync

import (
	"bytes"
	"fmt"
	"github.com/charmbracelet/bubbles/progress"
	"github.com/charmbracelet/lipgloss"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Repository represents the state of a GitHub repository
type Repository struct {
	Name string
	Done bool
	Err  error
}

// Model represents the state of the Bubble Tea program
type Model struct {
	Org          string
	Repositories []Repository
	Done         bool
	Errors       []error
	Progress     progress.Model
}

var (
	pendingStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("#FFA500")) // Orange
	doneStyle    = lipgloss.NewStyle().Foreground(lipgloss.Color("#00FF00")) // Green
	errorStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("#FF0000")) // Red
)

const (
	padding  = 2
	maxWidth = 80
)

// NewModel creates a new instance of the Bubble Tea Model
func NewModel(org string) Model {
	progressBar := progress.New(progress.WithDefaultGradient(), progress.WithScaledGradient("#FFA500", "#00FF00")) // Gradient from orange to green
	return Model{
		Org:      org,
		Progress: progressBar,
	}

}

// Init is the initial command for Bubble Tea
func (m Model) Init() tea.Cmd {
	return m.fetchRepositories
}

// Update processes messages and updates the state of the Model
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		if msg.String() == "q" {
			return m, tea.Quit
		}
	case tea.WindowSizeMsg:
		m.Progress.Width = msg.Width - padding*2 - 4
		if m.Progress.Width > maxWidth {
			m.Progress.Width = maxWidth
		}
		return m, nil
	case repositoriesFetchedMsg:
		m.Repositories = msg.Repositories
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

		// Calculate the number of completed repositories
		completed := 0
		for _, repo := range m.Repositories {
			if repo.Done {
				completed++
			}
		}

		// Update progress based on completed repositories
		m.Progress.SetPercent(float64(completed) / float64(len(m.Repositories)))

		// Check if all repositories are done
		m.Done = completed == len(m.Repositories)
		if m.Done {
			return m, tea.Quit
		}
		return m, nil

	case progress.FrameMsg:
		// Handle progress bar animation
		progressModel, cmd := m.Progress.Update(msg)
		m.Progress = progressModel.(progress.Model)
		return m, cmd
	}

	return m, nil
}

// View renders the Model to the terminal
func (m Model) View() string {
	var builder strings.Builder
	builder.WriteString("\n" + m.Progress.View() + "\n\n") // Render the progress bar

	if m.Done {
		builder.WriteString("All repositories processed.\n\n")
		for _, repo := range m.Repositories {
			if repo.Err != nil {
				builder.WriteString(errorStyle.Render(fmt.Sprintf("%s: %v\n", repo.Name, repo.Err)))
			} else {
				builder.WriteString(doneStyle.Render(fmt.Sprintf("%s: Done\n", repo.Name)))
			}
		}
	} else {
		builder.WriteString(fmt.Sprintf("Repositories in organization '%s':\n", m.Org))
		for _, repo := range m.Repositories {
			status := pendingStyle.Render("Pending")
			if repo.Done {
				if repo.Err != nil {
					status = errorStyle.Render(fmt.Sprintf("Error: %v", repo.Err))
				} else {
					status = doneStyle.Render("Done")
				}
			}
			builder.WriteString(fmt.Sprintf("%s: %s\n", repo.Name, status))
		}
	}
	builder.WriteString("\nPress 'q' to quit.")
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
