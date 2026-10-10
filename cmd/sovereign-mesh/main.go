package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"syscall"
	"time"

	"github.com/ajejfiejof/sovereign-mesh/pkg/crypto"
	"github.com/ajejfiejof/sovereign-mesh/pkg/p2p"
	"github.com/ajejfiejof/sovereign-mesh/pkg/storage"
	"github.com/ajejfiejof/sovereign-mesh/pkg/web"
)

const Version = "v0.1.0"

func defaultDataDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".sovereign-mesh"
	}
	return filepath.Join(home, ".sovereign-mesh")
}

func main() {
	if len(os.Args) < 2 {
		printUsage()
		os.Exit(1)
	}

	command := os.Args[1]

	switch command {
	case "node":
		runNode(os.Args[2:])
	case "keygen":
		runKeygen(os.Args[2:])
	case "post":
		runPost(os.Args[2:])
	case "bench":
		runBench(os.Args[2:])
	case "peers":
		runPeers(os.Args[2:])
	case "version":
		fmt.Printf("sovereign-mesh %s (Go P2P Overlay Daemon)\n", Version)
	default:
		fmt.Printf("Unknown command: %s\n\n", command)
		printUsage()
		os.Exit(1)
	}
}

func printUsage() {
	fmt.Println(`Sovereign Mesh: Self-Sovereign P2P Communication Overlay & Imageboard Daemon

Usage:
  sovereign-mesh <command> [options]

Commands:
  node      Start the P2P overlay daemon and web imageboard portal
  keygen    Generate a new self-sovereign Ed25519 identity keypair
  post      Mine Argon2id PoW and publish a post from the CLI
  peers     Query and display connected overlay peers
  bench     Benchmark Argon2id memory-hard mining performance
  version   Show version information

Run 'sovereign-mesh <command> -h' for command-specific options.`)
}

func runNode(args []string) {
	fs := flag.NewFlagSet("node", flag.ExitOnError)
	p2pPort := fs.Int("p2p", 9000, "TCP port for P2P overlay listener")
	webPort := fs.Int("web", 8080, "HTTP port for web imageboard portal")
	difficulty := fs.Int("diff", 2, "Argon2id PoW difficulty (leading hex zeros)")
	dataDir := fs.String("data", defaultDataDir(), "Data directory for database and keys")
	bootstrapPeer := fs.String("peer", "", "Bootstrap peer endpoint to connect to (host:port)")
	fs.Parse(args)

	if err := os.MkdirAll(*dataDir, 0700); err != nil {
		log.Fatalf("Failed to create data directory: %v", err)
	}

	keyFile := filepath.Join(*dataDir, "identity.json")
	id, err := crypto.LoadFromFile(keyFile)
	if err != nil {
		log.Println("[Init] No existing identity found. Generating fresh Ed25519 identity...")
		id, err = crypto.GenerateIdentity()
		if err != nil {
			log.Fatalf("Failed to generate identity: %v", err)
		}
		if err := id.SaveToFile(keyFile); err != nil {
			log.Fatalf("Failed to save identity to %s: %v", keyFile, err)
		}
		log.Printf("[Init] Identity saved to %s", keyFile)
	}

	dbFile := filepath.Join(*dataDir, "mesh.db")
	store, err := storage.Open(dbFile)
	if err != nil {
		log.Fatalf("Failed to open storage database: %v", err)
	}
	defer store.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		log.Println("\n[Shutdown] Terminating overlay node gracefully...")
		cancel()
	}()

	node := p2p.NewNode(id, store, *p2pPort, *difficulty)
	if err := node.Start(ctx); err != nil {
		log.Fatalf("Failed to start P2P node: %v", err)
	}

	if *bootstrapPeer != "" {
		go func() {
			time.Sleep(500 * time.Millisecond)
			log.Printf("[P2P] Connecting to bootstrap peer: %s", *bootstrapPeer)
			if err := node.ConnectPeer(*bootstrapPeer); err != nil {
				log.Printf("[P2P] Warning: Could not connect to bootstrap peer: %v", err)
			}
		}()
	}

	webServer := web.NewServer(node, store, id, *webPort)
	if err := webServer.Start(ctx); err != nil {
		log.Fatalf("Failed to start web server: %v", err)
	}

	fmt.Println()
	fmt.Println("=======================================================")
	fmt.Printf("  SOVEREIGN MESH OVERLAY NODE ACTIVE (%s)\n", Version)
	fmt.Printf("  Node Address:   %s\n", id.Address)
	fmt.Printf("  P2P Listener:   0.0.0.0:%d\n", *p2pPort)
	fmt.Printf("  Web Portal:     http://127.0.0.1:%d\n", *webPort)
	fmt.Printf("  PoW Difficulty: %d (Argon2id 32MB)\n", *difficulty)
	fmt.Println("=======================================================")
	fmt.Println()

	<-ctx.Done()
	time.Sleep(200 * time.Millisecond)
	log.Println("[Shutdown] Node offline.")
}

func runKeygen(args []string) {
	fs := flag.NewFlagSet("keygen", flag.ExitOnError)
	outFile := fs.String("out", "", "Output file path (default prints to stdout)")
	fs.Parse(args)

	id, err := crypto.GenerateIdentity()
	if err != nil {
		log.Fatalf("Key generation failed: %v", err)
	}

	if *outFile != "" {
		if err := id.SaveToFile(*outFile); err != nil {
			log.Fatalf("Failed to save key to %s: %v", *outFile, err)
		}
		fmt.Printf("New identity generated and saved to %s\n", *outFile)
	}

	fmt.Println("Cryptographic Address:   ", id.Address)
	fmt.Println("Public Key (Ed25519 Hex):", id.PubKeyHex)
	fmt.Println("Private Key (Secret Hex):", id.PrivKeyHex)
}

func runPost(args []string) {
	fs := flag.NewFlagSet("post", flag.ExitOnError)
	webPort := fs.Int("port", 8080, "Web API port of running node")
	board := fs.String("board", "g", "Board slug to post to")
	title := fs.String("title", "", "Thread title (optional)")
	content := fs.String("content", "", "Message text content")
	threadID := fs.Int64("thread", 0, "Thread ID to reply to (0 for new thread)")
	fs.Parse(args)

	if *content == "" {
		log.Fatal("Error: -content is required")
	}

	apiURL := fmt.Sprintf("http://%s/api/posts", net.JoinHostPort("127.0.0.1", strconv.Itoa(*webPort)))

	payload, _ := json.Marshal(map[string]interface{}{
		"board_slug": *board,
		"thread_id":  *threadID,
		"title":      *title,
		"content":    *content,
	})

	fmt.Printf("Publishing to /%s/ via %s (Mining PoW locally)...\n", *board, apiURL)
	start := time.Now()

	req, err := http.NewRequest("POST", apiURL, bytes.NewReader(payload))
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "SovereignMesh-CLI/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to publish post: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Node rejected post (HTTP %d)", resp.StatusCode)
	}

	var res map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&res)

	fmt.Printf("Post published successfully in %v!\n", time.Since(start))
	fmt.Printf("Post ID:   %v\n", res["post_id"])
	fmt.Printf("Thread ID: %v\n", res["thread_id"])
	fmt.Printf("PoW Hash:  %v (Nonce: %v)\n", res["pow_hash"], res["nonce"])
}

func runPeers(args []string) {
	fs := flag.NewFlagSet("peers", flag.ExitOnError)
	webPort := fs.Int("port", 8080, "Web API port of running node")
	fs.Parse(args)

	apiURL := fmt.Sprintf("http://%s/api/node/info", net.JoinHostPort("127.0.0.1", strconv.Itoa(*webPort)))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		log.Fatalf("Failed to create request: %v", err)
	}
	req.Header.Set("User-Agent", "SovereignMesh-CLI/"+Version)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Fatalf("Failed to query node info: %v", err)
	}
	defer resp.Body.Close()

	var info map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&info)

	fmt.Println("Connected Overlay Node Info:")
	fmt.Printf("  Address:      %v\n", info["address"])
	fmt.Printf("  P2P Port:     %v\n", info["p2p_port"])
	fmt.Printf("  PoW Target:   %v\n", info["difficulty"])
	fmt.Printf("  Active Peers: %v\n", info["active_peers"])
	if list, ok := info["peer_list"].([]interface{}); ok && len(list) > 0 {
		fmt.Println("  Peers:")
		for _, p := range list {
			fmt.Printf("    - %v\n", p)
		}
	}
}

func runBench(args []string) {
	fs := flag.NewFlagSet("bench", flag.ExitOnError)
	fs.Parse(args)

	payload := "SovereignMesh:Benchmark:SamplePayload"
	fmt.Println("Benchmarking Argon2id Memory-Hard Proof-of-Work (32 MB RAM per hash attempt):")

	difficulties := []int{1, 2}
	for _, diff := range difficulties {
		fmt.Printf("Mining Difficulty %d (prefix '%s')...\n", diff, string(bytes.Repeat([]byte("0"), diff)))
		res, err := crypto.Mine(payload, diff)
		if err != nil {
			log.Fatalf("Mining failed: %v", err)
		}
		rate := float64(res.Nonce+1) / res.Duration.Seconds()
		fmt.Printf("  -> Solved in %v! Nonce: %d | Hash: %s | Speed: %.1f H/s\n",
			res.Duration, res.Nonce, res.Hash[:12]+"...", rate)
	}
}
