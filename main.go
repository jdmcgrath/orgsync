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
		help bool
	)

	// Set up flag usage
	flag.BoolVar(&help, "help", false, "Show this help message")

	// Customize usage message
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] org\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nSynchronize all repositories for a given GitHub organization.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExample:\n")
		fmt.Fprintf(os.Stderr, "  %s my-org\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "\nDependencies:\n")
		fmt.Fprintf(os.Stderr, "  This program requires the GitHub CLI (`gh`) to be installed and authenticated.\n")
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

	// Initialize the Bubble Tea program
	p := tea.NewProgram(sync.NewModel(org))

	// Run the program and handle errors
	if _, err := p.Run(); err != nil {
		log.Fatalf("Error: %v\n", err)
	}

	// Log the completion of the synchronization process
	log.Printf("Synchronization completed for organization: %s\n", org)
}
