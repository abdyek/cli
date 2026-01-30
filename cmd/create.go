package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/umono-cms/cli/internal/project"
	"golang.org/x/term"
)

var createCmd = &cobra.Command{
	Use:   "create <project-name>",
	Short: "Create a new project",
	Long: `Create a new Umono CMS project with the latest release.

This command will:
  - Download the latest Umono release for your platform
  - Extract it to a new directory
  - Set up initial configuration with your credentials

Example:
  umono create my-project
  cd my-project
  umono up`,
	Args: cobra.ExactArgs(1),
	Run:  runCreate,
}

func init() {
	rootCmd.AddCommand(createCmd)
}

func runCreate(cmd *cobra.Command, args []string) {
	projectName := args[0]

	wd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get working directory: %v\n", err)
		os.Exit(1)
	}
	projectPath := filepath.Join(wd, projectName)

	err = os.Mkdir(projectPath, 0o755)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to create project directory: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("📦 Creating new Umono project: '%s'\n", projectName)
	fmt.Printf("   Configure root account credentials (you can change these later)\n\n")

	fmt.Print("Username: ")
	var username string
	fmt.Scanln(&username)

	fmt.Print("Password: ")
	passwordBytes, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read password: %v\n", err)
		os.Exit(1)
	}
	password := strings.TrimSpace(string(passwordBytes))
	fmt.Println()

	var port string
	for {
		fmt.Print("Port [8999]: ")
		var portInput string
		fmt.Scanln(&portInput)
		portInput = strings.TrimSpace(portInput)

		if portInput == "" {
			port = "8999"
			break
		}

		portNum, err := strconv.Atoi(portInput)
		if err != nil {
			fmt.Println("   ⚠️  Please enter a valid number")
			continue
		}

		if portNum < 1 || portNum > 65535 {
			fmt.Println("   ⚠️  Port must be between 1 and 65535")
			continue
		}

		if portNum < 1024 {
			fmt.Println("   ⚠️  Ports below 1024 require root privileges")
			continue
		}

		port = portInput
		break
	}
	fmt.Println()

	// TODO: Use port
	_ = port

	err = project.Create(cmd, projectPath, username, password)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✅ Project '%s' created successfully!\n\n", projectName)
	fmt.Println("Next steps:")
	fmt.Printf("  cd %s\n", projectName)
	fmt.Println("  umono up")
	fmt.Println()
}
