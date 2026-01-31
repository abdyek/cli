package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/spf13/cobra"
)

var downCmd = &cobra.Command{
	Use:   "down",
	Short: "Stop Umono",
	Long:  `Stop the running Umono application in the current directory.`,
	Run:   runDown,
}

func init() {
	rootCmd.AddCommand(downCmd)
}

func runDown(cmd *cobra.Command, args []string) {
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to get current directory: %v\n", err)
		os.Exit(1)
	}

	pidPath := filepath.Join(cwd, ".PID")

	pidData, err := os.ReadFile(pidPath)
	if os.IsNotExist(err) {
		fmt.Println("Umono is not running (no .PID file found)")
		os.Exit(0)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to read .PID file: %v\n", err)
		os.Exit(1)
	}

	pidStr := strings.TrimSpace(string(pidData))
	pid, err := strconv.Atoi(pidStr)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: invalid PID in .PID file: %s\n", pidStr)
		os.Remove(pidPath)
		os.Exit(1)
	}

	process, err := os.FindProcess(pid)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to find process: %v\n", err)
		os.Remove(pidPath)
		os.Exit(1)
	}

	if err := process.Signal(syscall.Signal(0)); err != nil {
		fmt.Println("Umono is not running (stale .PID file removed)")
		os.Remove(pidPath)
		os.Exit(0)
	}

	if err := process.Signal(syscall.SIGTERM); err != nil {
		fmt.Fprintf(os.Stderr, "Error: failed to stop umono: %v\n", err)
		os.Exit(1)
	}

	os.Remove(pidPath)

	fmt.Println("Umono stopped (PID:", pid, ")")
}
