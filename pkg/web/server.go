package web

import (
	"context"
	_ "embed"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ajejfiejof/sovereign-mesh/pkg/crypto"
	"github.com/ajejfiejof/sovereign-mesh/pkg/p2p"
	"github.com/ajejfiejof/sovereign-mesh/pkg/storage"
)

//go:embed static/index.html
var indexHTML []byte

type Server struct {
	node       *p2p.Node
	store      *storage.DB
	identity   *crypto.Identity
	port       int
	httpServer *http.Server
}

func NewServer(node *p2p.Node, store *storage.DB, id *crypto.Identity, port int) *Server {
	return &Server{
		node:     node,
		store:    store,
		identity: id,
		port:     port,
	}
}

func (s *Server) Start(ctx context.Context) error {
	mux := http.NewServeMux()

	// API Routes
	mux.HandleFunc("/api/node/info", s.handleNodeInfo)
	mux.HandleFunc("/api/boards", s.handleBoards)
	mux.HandleFunc("/api/threads", s.handleThreads)
	mux.HandleFunc("/api/posts", s.handlePosts)
	mux.HandleFunc("/api/peers", s.handlePeers)
	mux.HandleFunc("/api/peers/connect", s.handleConnectPeer)

	// Web Frontend
	mux.HandleFunc("/", s.handleIndex)

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(s.port))
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: corsMiddleware(mux),
	}

	log.Printf("[Web] Decentralized Imageboard Portal online at http://127.0.0.1:%d", s.port)

	go func() {
		<-ctx.Done()
		_ = s.httpServer.Shutdown(context.Background())
	}()

	go func() {
		if err := s.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("[Web] Server error: %v", err)
		}
	}()

	return nil
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func (s *Server) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	peers := s.node.GetActivePeers()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"address":      s.identity.Address,
		"pubkey":       s.identity.PubKeyHex,
		"p2p_port":     s.node.Port,
		"difficulty":   s.node.Difficulty,
		"active_peers": len(peers),
		"peer_list":    peers,
		"status":       "online",
	})
}

func (s *Server) handleBoards(w http.ResponseWriter, r *http.Request) {
	boards, err := s.store.GetBoards()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, boards)
}

func (s *Server) handleThreads(w http.ResponseWriter, r *http.Request) {
	board := r.URL.Query().Get("board")
	if board == "" {
		board = "g"
	}
	threads, err := s.store.GetThreads(board, 50)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, threads)
}

func (s *Server) handlePosts(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet {
		threadIDStr := r.URL.Query().Get("thread_id")
		tID, err := strconv.ParseInt(threadIDStr, 10, 64)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid thread_id"})
			return
		}
		posts, err := s.store.GetPosts(tID)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, posts)
		return
	}

	if r.Method == http.MethodPost {
		var req struct {
			BoardSlug string `json:"board_slug"`
			ThreadID  int64  `json:"thread_id"`
			ThreadRef string `json:"thread_ref"`
			Title     string `json:"title"`
			Content   string `json:"content"`
		}

		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json payload"})
			return
		}

		if strings.TrimSpace(req.Content) == "" {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "content cannot be empty"})
			return
		}

		if req.BoardSlug == "" {
			req.BoardSlug = "g"
		}

		// Mine Proof of Work on behalf of the node
		powRes, err := crypto.Mine(req.Content, s.node.Difficulty)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pow mining failed"})
			return
		}

		// Sign with node identity
		sig := s.identity.Sign([]byte(req.Content))

		post := &storage.Post{
			BoardSlug:     req.BoardSlug,
			ThreadID:      req.ThreadID,
			ThreadRef:     req.ThreadRef,
			Title:         req.Title,
			AuthorAddress: s.identity.Address,
			AuthorPubKey:  s.identity.PubKeyHex,
			Content:       req.Content,
			Nonce:         powRes.Nonce,
			PoWHash:       powRes.Hash,
			Signature:     sig,
			Timestamp:     time.Now().Unix(),
		}

		pID, tID, err := s.store.CreatePost(post)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
			return
		}

		post.ID = pID
		post.ThreadID = tID

		// Gossip to network peers
		s.node.BroadcastPost(post)

		writeJSON(w, http.StatusOK, map[string]interface{}{
			"status":    "published",
			"post_id":   pID,
			"thread_id": tID,
			"nonce":     powRes.Nonce,
			"pow_hash":  powRes.Hash,
			"duration":  powRes.Duration.String(),
		})
		return
	}

	w.WriteHeader(http.StatusMethodNotAllowed)
}

func (s *Server) handlePeers(w http.ResponseWriter, r *http.Request) {
	peers, err := s.store.GetPeers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, peers)
}

func (s *Server) handleConnectPeer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Endpoint string `json:"endpoint"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid endpoint"})
		return
	}

	if err := s.node.ConnectPeer(req.Endpoint); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"status": "connected", "endpoint": req.Endpoint})
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(indexHTML)
}
