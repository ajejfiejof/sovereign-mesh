package main

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/ajejfiejof/sovereign-mesh/pkg/crypto"
	"github.com/ajejfiejof/sovereign-mesh/pkg/p2p"
	"github.com/ajejfiejof/sovereign-mesh/pkg/storage"
	"github.com/ajejfiejof/sovereign-mesh/pkg/web"
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

	// 5. Test post and peers against a running test node
	id, _ := crypto.GenerateIdentity()
	store, err := storage.Open(filepath.Join(tempDir, "cli_test.db"))
	if err != nil {
		t.Fatalf("Storage open failed: %v", err)
	}
	defer store.Close()

	node := p2p.NewNode(id, store, 9301, 1)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	_ = node.Start(ctx)

	webPort := 9302
	webServer := web.NewServer(node, store, id, webPort)
	_ = webServer.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	// Run CLI post command
	runPost([]string{
		"-port", "9302",
		"-board", "g",
		"-title", "CLI Post Test",
		"-content", "Posting directly through the CLI tool.",
	})

	// Run CLI peers command
	runPeers([]string{"-port", "9302"})
}
