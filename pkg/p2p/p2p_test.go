package p2p

import (
	"bytes"
	"context"
	"net"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ajejfiejof/sovereign-mesh/pkg/crypto"
	"github.com/ajejfiejof/sovereign-mesh/pkg/storage"
)

func TestWireProtocolFraming(t *testing.T) {
	var buf bytes.Buffer
	testPayload := []byte("Protocol framing test payload")

	// 1. Valid write and read
	if err := WritePacket(&buf, MsgTypeGossipPost, testPayload); err != nil {
		t.Fatalf("WritePacket failed: %v", err)
	}

	pkt, err := ReadPacket(&buf)
	if err != nil {
		t.Fatalf("ReadPacket failed: %v", err)
	}
	if pkt.Type != MsgTypeGossipPost || string(pkt.Payload) != string(testPayload) {
		t.Errorf("Packet mismatch: got type=%x, payload=%s", pkt.Type, string(pkt.Payload))
	}

	// 2. Invalid magic bytes
	invalidBuf := bytes.NewReader([]byte{0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x00})
	if _, err := ReadPacket(invalidBuf); err == nil {
		t.Error("Expected error on invalid magic bytes, got nil")
	}

	// 3. Oversized payload error (exceeds 4MB)
	oversizedBuf := bytes.NewReader([]byte{
		0x53, 0x4D, // Magic
		0x01,       // Type
		0x01, 0x00, 0x00, 0x00, // Length = 16 MB
	})
	if _, err := ReadPacket(oversizedBuf); err == nil {
		t.Error("Expected error on oversized payload length, got nil")
	}
}

func TestP2PPeeringAndGossipReplication(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Setup Node 1
	id1, _ := crypto.GenerateIdentity()
	store1, err := storage.Open(filepath.Join(tempDir, "node1.db"))
	if err != nil {
		t.Fatalf("Open node1.db failed: %v", err)
	}
	defer store1.Close()

	node1 := NewNode(id1, store1, 9111, 1) // Difficulty=1 for instant test execution
	ctx1, cancel1 := context.WithCancel(context.Background())
	defer cancel1()
	if err := node1.Start(ctx1); err != nil {
		t.Fatalf("Node 1 start failed: %v", err)
	}

	// 2. Setup Node 2
	id2, _ := crypto.GenerateIdentity()
	store2, err := storage.Open(filepath.Join(tempDir, "node2.db"))
	if err != nil {
		t.Fatalf("Open node2.db failed: %v", err)
	}
	defer store2.Close()

	node2 := NewNode(id2, store2, 9112, 1)
	ctx2, cancel2 := context.WithCancel(context.Background())
	defer cancel2()
	if err := node2.Start(ctx2); err != nil {
		t.Fatalf("Node 2 start failed: %v", err)
	}

	time.Sleep(50 * time.Millisecond)

	// 3. Connect Node 1 -> Node 2 using net.JoinHostPort
	targetEndpoint := net.JoinHostPort("127.0.0.1", strconv.Itoa(9112))
	if err := node1.ConnectPeer(targetEndpoint); err != nil {
		t.Fatalf("ConnectPeer failed: %v", err)
	}

	// Wait for mutual handshake & PEX
	time.Sleep(100 * time.Millisecond)

	// Check active peers list
	activePeers := node1.GetActivePeers()
	if len(activePeers) != 1 {
		t.Errorf("Expected 1 active peer on Node 1, got %d", len(activePeers))
	}

	// Test PruneAllBoards maintenance invocation
	node1.PruneAllBoards(10)

	// 4. Create and mine a post on Node 1
	content := "Censorship-resistant P2P gossiping verified."
	powRes, err := crypto.Mine(content, 1)
	if err != nil {
		t.Fatalf("Mine failed: %v", err)
	}
	sig := id1.Sign([]byte(content))

	post := &storage.Post{
		BoardSlug:     "g",
		Title:         "Gossip Test",
		AuthorAddress: id1.Address,
		AuthorPubKey:  id1.PubKeyHex,
		Content:       content,
		Nonce:         powRes.Nonce,
		PoWHash:       powRes.Hash,
		Signature:     sig,
		Timestamp:     time.Now().Unix(),
	}

	pID, tID, err := store1.CreatePost(post)
	if err != nil {
		t.Fatalf("CreatePost on Node 1 failed: %v", err)
	}
	post.ID = pID
	post.ThreadID = tID

	// Broadcast from Node 1
	node1.BroadcastPost(post)

	// Wait for network gossip
	time.Sleep(150 * time.Millisecond)

	// 5. Verify Node 2 ingested the gossiped post
	threads2, err := store2.GetThreads("g", 10)
	if err != nil {
		t.Fatalf("GetThreads on Node 2 failed: %v", err)
	}

	if len(threads2) == 0 {
		t.Fatal("Node 2 did not receive or ingest gossiped thread")
	}

	posts2, err := store2.GetPosts(threads2[0].ID)
	if err != nil || len(posts2) == 0 {
		t.Fatalf("Failed to retrieve posts from Node 2: %v", err)
	}

	if posts2[0].Content != content {
		t.Errorf("Content mismatch on Node 2: got %q, expected %q", posts2[0].Content, content)
	}
}
