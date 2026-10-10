package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ajejfiejof/sovereign-mesh/pkg/crypto"
	"github.com/ajejfiejof/sovereign-mesh/pkg/p2p"
	"github.com/ajejfiejof/sovereign-mesh/pkg/storage"
)

func TestWebServerEndpoints(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "web_test.db")

	id, _ := crypto.GenerateIdentity()
	store, err := storage.Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	node := p2p.NewNode(id, store, 9201, 1) // PoW difficulty 1 for fast test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = node.Start(ctx)

	webPort := 9202
	server := NewServer(node, store, id, webPort)
	_ = server.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", net.JoinHostPort("127.0.0.1", strconv.Itoa(webPort)))

	// 1. Test /api/boards
	resp, err := http.Get(baseURL + "/api/boards")
	if err != nil {
		t.Fatalf("GET /api/boards failed: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}

	var boards []storage.Board
	if err := json.NewDecoder(resp.Body).Decode(&boards); err != nil || len(boards) < 4 {
		t.Errorf("Failed to decode boards: %v", err)
	}

	// 2. Test /api/node/info
	infoResp, err := http.Get(baseURL + "/api/node/info")
	if err != nil {
		t.Fatalf("GET /api/node/info failed: %v", err)
	}
	defer infoResp.Body.Close()

	if infoResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", infoResp.StatusCode)
	}

	// 3. Test POST /api/posts
	postBody, _ := json.Marshal(map[string]interface{}{
		"board_slug": "g",
		"title":      "Decentralized Web Post Test",
		"content":    "Testing automatic node-side mining and storage.",
	})

	postResp, err := http.Post(baseURL+"/api/posts", "application/json", bytes.NewReader(postBody))
	if err != nil {
		t.Fatalf("POST /api/posts failed: %v", err)
	}
	defer postResp.Body.Close()

	if postResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", postResp.StatusCode)
	}

	var postResult map[string]interface{}
	if err := json.NewDecoder(postResp.Body).Decode(&postResult); err != nil {
		t.Fatalf("Failed to decode post result: %v", err)
	}

	if postResult["status"] != "published" {
		t.Errorf("Expected status published, got %v", postResult["status"])
	}
}
