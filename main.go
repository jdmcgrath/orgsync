package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jdmcgrath/orgsync/sync"
)

func main() {
	// Define flags
	var (
		help         bool
		testMode     bool
		testRepos    int
		testFailRate float64
	)

	// Set up flag usage
	flag.BoolVar(&help, "help", false, "Show this help message")
	flag.BoolVar(&testMode, "test", false, "Run in test mode with simulated operations")
	flag.IntVar(&testRepos, "test-repos", 20, "Number of test repositories to simulate")
	flag.Float64Var(&testFailRate, "test-fail-rate", 0.1, "Failure rate for test mode (0.0-1.0)")

	// Customize usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] org\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nSynchronize all repositories for a given GitHub organization.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  %s my-org\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --test test-org              # Run in test mode\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  %s --test --test-repos=50 test  # Test with 50 repos\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nControls:\n")
		fmt.Fprintf(os.Stderr, "  q - Quit/Cancel\n")
		fmt.Fprintf(os.Stderr, "  c - Toggle showing completed repositories\n")
		fmt.Fprintf(os.Stderr, "\nDependencies:\n")
		fmt.Fprintf(os.Stderr, "  This program requires the GitHub CLI (`gh`) to be installed and authenticated.\n")
		fmt.Fprintf(os.Stderr, "\nTest Mode:\n")
		fmt.Fprintf(os.Stderr, "  Test mode simulates repository operations without creating actual git\n")
		fmt.Fprintf(os.Stderr, "  repositories. This is useful for testing the UI and visualizations.\n")
	}

	// Parse arguments
	flag.Parse()

	// Show help message if requested
	if help {
		flag.Usage()
		os.Exit(0)
	}

	// Ensure organization name is provided
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	// Retrieve the organization name
	org := flag.Arg(0)
	if org == "" {
		log.Fatalf("Error: organization name must not be empty")
	}

	// Log the start of the synchronization process
	log.Printf("Starting synchronization for organization: %s\n", org)
	if testMode {
		log.Printf("Running in TEST MODE with %d simulated repositories\n", testRepos)
	}

	// Initialize the model
	model := sync.NewModel(org)
	if testMode {
		model.TestMode = true
		model.TestRepoCount = testRepos
		model.TestFailRate = testFailRate
	}

	// Initialize the Bubble Tea program
	p := tea.NewProgram(model, tea.WithAltScreen())

	// Run the program and handle errors
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error: %v\n", err)
	}

	// Log the completion of the synchronization process
	log.Printf("Synchronization completed for organization: %s\n", org)
}
