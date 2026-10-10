package web

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	node := p2p.NewNode(id, store, 9211, 1) // PoW difficulty 1 for fast test
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_ = node.Start(ctx)

	webPort := 9212
	server := NewServer(node, store, id, webPort)
	_ = server.Start(ctx)

	time.Sleep(100 * time.Millisecond)

	baseURL := fmt.Sprintf("http://%s", net.JoinHostPort("127.0.0.1", strconv.Itoa(webPort)))

	// 1. Test GET / (HTML Index)
	indexResp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer indexResp.Body.Close()
	if indexResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for index, got %d", indexResp.StatusCode)
	}
	bodyBytes, _ := io.ReadAll(indexResp.Body)
	if !bytes.Contains(bodyBytes, []byte("SOVEREIGN MESH")) {
		t.Error("Index HTML does not contain SOVEREIGN MESH title")
	}

	// 2. Test GET /api/boards
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

	// 3. Test GET /api/node/info
	infoResp, err := http.Get(baseURL + "/api/node/info")
	if err != nil {
		t.Fatalf("GET /api/node/info failed: %v", err)
	}
	defer infoResp.Body.Close()

	if infoResp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", infoResp.StatusCode)
	}

	// 4. Test POST /api/posts (Publish OP Post)
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

	threadID := int64(postResult["thread_id"].(float64))

	// 5. Test GET /api/threads?board=g
	threadsResp, err := http.Get(baseURL + "/api/threads?board=g")
	if err != nil {
		t.Fatalf("GET /api/threads failed: %v", err)
	}
	defer threadsResp.Body.Close()
	var threads []storage.Thread
	if err := json.NewDecoder(threadsResp.Body).Decode(&threads); err != nil || len(threads) == 0 {
		t.Errorf("Failed to decode threads or empty: %v", err)
	}

	// 6. Test GET /api/posts?thread_id=...
	getPostsResp, err := http.Get(fmt.Sprintf("%s/api/posts?thread_id=%d", baseURL, threadID))
	if err != nil {
		t.Fatalf("GET /api/posts failed: %v", err)
	}
	defer getPostsResp.Body.Close()
	var posts []storage.Post
	if err := json.NewDecoder(getPostsResp.Body).Decode(&posts); err != nil || len(posts) == 0 {
		t.Errorf("Failed to decode posts or empty: %v", err)
	}

	// 7. Test GET /api/peers
	peersResp, err := http.Get(baseURL + "/api/peers")
	if err != nil {
		t.Fatalf("GET /api/peers failed: %v", err)
	}
	defer peersResp.Body.Close()
	if peersResp.StatusCode != http.StatusOK {
		t.Errorf("Expected 200 for peers, got %d", peersResp.StatusCode)
	}

	// 8. Test POST /api/peers/connect (Valid and Invalid)
	connPayload, _ := json.Marshal(map[string]string{"endpoint": "127.0.0.1:9299"})
	connResp, err := http.Post(baseURL+"/api/peers/connect", "application/json", bytes.NewReader(connPayload))
	if err != nil {
		t.Fatalf("POST /api/peers/connect failed: %v", err)
	}
	defer connResp.Body.Close()
	// Connection will fail since no peer is at 9299, expecting 500 error response
	if connResp.StatusCode != http.StatusInternalServerError {
		t.Errorf("Expected status 500 for unreachable peer, got %d", connResp.StatusCode)
	}

	// 9. Error cases: empty content
	badPostBody, _ := json.Marshal(map[string]interface{}{"content": ""})
	badResp, err := http.Post(baseURL+"/api/posts", "application/json", bytes.NewReader(badPostBody))
	if err != nil || badResp.StatusCode != http.StatusBadRequest {
		t.Errorf("Expected 400 for empty content, got %d", badResp.StatusCode)
	}
}
