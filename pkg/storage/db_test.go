package storage

import (
	"fmt"
	"path/filepath"
	"testing"
	"time"
)

func TestStorageLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_mesh.db")

	store, err := Open(dbPath)
	if err != nil {
		t.Fatalf("Open failed: %v", err)
	}
	defer store.Close()

	// 1. Verify default boards
	boards, err := store.GetBoards()
	if err != nil {
		t.Fatalf("GetBoards failed: %v", err)
	}
	if len(boards) < 4 {
		t.Errorf("Expected at least 4 default boards, got %d", len(boards))
	}

	// 2. Create OP Post
	opPost := &Post{
		BoardSlug:     "g",
		Title:         "Building a Sovereign P2P Web",
		AuthorAddress: "mesh:testauthor123456789",
		AuthorPubKey:  "deadbeef",
		Content:       "Can we build a truly decentralized overlay network?",
		Nonce:         42,
		PoWHash:       "00abcdef",
		Signature:     "validsig",
		Timestamp:     time.Now().Unix(),
	}

	pID, tID, err := store.CreatePost(opPost)
	if err != nil {
		t.Fatalf("CreatePost (OP) failed: %v", err)
	}
	if pID == 0 || tID == 0 {
		t.Errorf("Invalid IDs returned: pID=%d, tID=%d", pID, tID)
	}

	// 3. Create Reply Post
	replyPost := &Post{
		ThreadID:      tID,
		BoardSlug:     "g",
		AuthorAddress: "mesh:peer987654321",
		AuthorPubKey:  "beefcafe",
		Content:       "Yes, by using cryptographic addresses and micro-PoW.",
		Nonce:         88,
		PoWHash:       "00fedcba",
		Signature:     "validsig2",
		Timestamp:     time.Now().Unix(),
	}

	rPID, rTID, err := store.CreatePost(replyPost)
	if err != nil {
		t.Fatalf("CreatePost (Reply) failed: %v", err)
	}
	if rTID != tID {
		t.Errorf("Reply thread ID mismatch: got %d, expected %d", rTID, tID)
	}
	if rPID <= pID {
		t.Errorf("Reply post ID should be greater than OP post ID")
	}

	// 4. Retrieve Threads
	threads, err := store.GetThreads("g", 10)
	if err != nil {
		t.Fatalf("GetThreads failed: %v", err)
	}
	if len(threads) != 1 {
		t.Fatalf("Expected 1 thread on /g/, got %d", len(threads))
	}
	if threads[0].PostCount != 2 {
		t.Errorf("Expected post count 2, got %d", threads[0].PostCount)
	}

	// Empty board threads
	emptyThreads, err := store.GetThreads("emptyboard", 10)
	if err != nil || len(emptyThreads) != 0 {
		t.Errorf("Expected 0 threads on empty board, got %d (err=%v)", len(emptyThreads), err)
	}

	// 5. Retrieve Posts
	posts, err := store.GetPosts(tID)
	if err != nil {
		t.Fatalf("GetPosts failed: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("Expected 2 posts in thread, got %d", len(posts))
	}
	if !posts[0].IsOP || posts[1].IsOP {
		t.Errorf("Post OP flags incorrect: posts[0].IsOP=%v, posts[1].IsOP=%v", posts[0].IsOP, posts[1].IsOP)
	}

	// 6. Test Thread Pruning (Ephemeral 4chan Model)
	// Create 4 more threads on /g/ (total 5) with unique PoWHash
	for i := 1; i <= 4; i++ {
		_, _, err := store.CreatePost(&Post{
			BoardSlug:     "g",
			Title:         fmt.Sprintf("Extra Thread %d", i),
			AuthorAddress: "mesh:author",
			Content:       "Post body",
			PoWHash:       fmt.Sprintf("pow_extra_%d", i),
			Signature:     "sig",
			Timestamp:     time.Now().Unix(),
		})
		if err != nil {
			t.Fatalf("Failed to create extra thread %d: %v", i, err)
		}
	}

	threadsBefore, _ := store.GetThreads("g", 20)
	if len(threadsBefore) != 5 {
		t.Errorf("Expected 5 threads before pruning, got %d", len(threadsBefore))
	}

	// Prune, keeping only the top 2
	prunedCount, err := store.PruneThreads("g", 2)
	if err != nil {
		t.Fatalf("PruneThreads failed: %v", err)
	}
	if prunedCount != 3 {
		t.Errorf("Expected 3 threads pruned, got %d", prunedCount)
	}

	threadsAfter, _ := store.GetThreads("g", 20)
	if len(threadsAfter) != 2 {
		t.Errorf("Expected 2 threads remaining after pruning, got %d", len(threadsAfter))
	}

	// 7. Test Peer Tracking
	if err := store.UpsertPeer("mesh:peernode1", "127.0.0.1:9001", true); err != nil {
		t.Fatalf("UpsertPeer failed: %v", err)
	}
	if err := store.UpsertPeer("mesh:peernode1", "127.0.0.1:9002", false); err != nil {
		t.Fatalf("UpsertPeer update failed: %v", err)
	}

	peers, err := store.GetPeers()
	if err != nil || len(peers) != 1 {
		t.Fatalf("Expected 1 peer, got %d (err=%v)", len(peers), err)
	}
	if peers[0].Endpoint != "127.0.0.1:9002" || peers[0].Connected {
		t.Errorf("Peer update failed to persist: %+v", peers[0])
	}
}
