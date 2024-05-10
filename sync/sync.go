package sync

import (
	"bytes"
	"fmt"
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
}

// NewModel creates a new instance of the Bubble Tea Model
func NewModel(org string) Model {
	return Model{Org: org}
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

	case repositoriesFetchedMsg:
		m.Repositories = msg.Repositories
		return m, tea.Batch(m.syncRepositories()...)

	case repositoryProcessedMsg:
		for i := range m.Repositories {
			if m.Repositories[i].Name == msg.Repo.Name {
				m.Repositories[i].Done = true
				m.Repositories[i].Err = msg.Err
			}
		}
		m.Done = true
		for _, repo := range m.Repositories {
			if !repo.Done {
				m.Done = false
				break
			}
		}
		if m.Done {
			return m, tea.Quit
		}
	}

	return m, nil
}

// View renders the Model to the terminal
func (m Model) View() string {
	var builder strings.Builder

	if m.Done {
		builder.WriteString("All repositories processed.\n\n")
		for _, repo := range m.Repositories {
			if repo.Err != nil {
				builder.WriteString(fmt.Sprintf("%s: %v\n", repo.Name, repo.Err))
			} else {
				builder.WriteString(fmt.Sprintf("%s: Done\n", repo.Name))
			}
		}
	} else {
		builder.WriteString(fmt.Sprintf("Repositories in organization '%s':\n", m.Org))
		for _, repo := range m.Repositories {
			status := "Pending"
			if repo.Done {
				status = "Done"
				if repo.Err != nil {
					status = fmt.Sprintf("Error: %v", repo.Err)
				}
			}
			builder.WriteString(fmt.Sprintf("%s: %s\n", repo.Name, status))
		}
		builder.WriteString("\nPress 'q' to quit.")
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
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to clone %s: %w", repo, err)
	}
	fmt.Printf("Cloned %s\n", repo)
	return nil
}

func fetchRepo(repoDir, repo string) error {
	cmd := exec.Command("git", "-C", repoDir, "fetch", "origin")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to fetch %s: %w", repo, err)
	}
	fmt.Printf("Fetched %s\n", repo)
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
