package main

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

func TestCLICommands(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Test keygen with output file
	keyFile := filepath.Join(tempDir, "test_identity.json")
	runKeygen([]string{"-out", keyFile})

	// 2. Test bench with default flags
	runBench([]string{})

	// 3. Test default data directory
	dataDir := defaultDataDir()
	if dataDir == "" {
		t.Error("defaultDataDir returned empty string")
	}

	// 4. Test usage output
	printUsage()

	// 5. Test runNode in background with SIGTERM shutdown
	nodeDone := make(chan struct{})
	go func() {
		defer close(nodeDone)
		runNode([]string{
			"-p2p", "9312",
			"-web", "9313",
			"-diff", "1",
			"-data", tempDir,
		})
	}()

	time.Sleep(200 * time.Millisecond)

	// Test runPost against the running node
	runPost([]string{
		"-port", "9313",
		"-board", "g",
		"-title", "CLI Post Test",
		"-content", "Posting directly through the CLI tool.",
	})

	// Test runPeers against the running node
	runPeers([]string{"-port", "9313"})

	// Trigger graceful shutdown of runNode via SIGTERM
	proc, err := os.FindProcess(os.Getpid())
	if err == nil {
		_ = proc.Signal(syscall.SIGTERM)
	}

	select {
	case <-nodeDone:
		// Graceful exit confirmed
	case <-time.After(3 * time.Second):
		t.Fatal("runNode timed out waiting for graceful shutdown")
	}
}
