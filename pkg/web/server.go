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

	"github.com/ajejfiejof/sovereign-mesh/pkg/antiscrape"
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
	shield     *antiscrape.Shield
	httpServer *http.Server
}

func NewServer(node *p2p.Node, store *storage.DB, id *crypto.Identity, port int) *Server {
	return &Server{
		node:     node,
		store:    store,
		identity: id,
		port:     port,
		shield:   antiscrape.NewShield(10.0, 30.0), // 10 req/sec, burst capacity 30
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
	mux.HandleFunc("/api/canary/", s.handleCanaryTrap)

	// Web Frontend
	mux.HandleFunc("/", s.handleIndex)

	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(s.port))
	s.httpServer = &http.Server{
		Addr:    addr,
		Handler: s.securityMiddleware(corsMiddleware(mux)),
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

// securityMiddleware runs anti-scraping heuristics, canary honeypots, and token-bucket checks.
func (s *Server) securityMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Only enforce anti-scraping on /api/ endpoints to avoid blocking asset loading
		if strings.HasPrefix(r.URL.Path, "/api/") && r.URL.Path != "/api/node/info" {
			allowed, tarpit, reason := s.shield.CheckRequest(r)
			if tarpit {
				log.Printf("[AntiScrape] Tarpitting detected scraper from %s (Path: %s, Reason: %s)", antiscrape.ExtractIP(r), r.URL.Path, reason)
				s.shield.TarpitResponse(w)
				return
			}
			if !allowed {
				w.Header().Set("X-Mesh-PoW-Challenge", "difficulty=2")
				writeJSON(w, http.StatusTooManyRequests, map[string]string{
					"error":   reason,
					"message": "Bulk automated reading throttled. Solve computational challenge to proceed.",
				})
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-Mesh-PoW-Solution")
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

func (s *Server) handleCanaryTrap(w http.ResponseWriter, r *http.Request) {
	// Scrapers accessing this endpoint get trapped by securityMiddleware
	w.WriteHeader(http.StatusNotFound)
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
		"antiscrape":   "active",
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

		// Apply zero-width invisible watermarking to served post content
		for i := range posts {
			posts[i].Content = antiscrape.WatermarkText(posts[i].Content, s.identity.Address)
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

		// Zero-Tolerance Moderation & Anti-Hate Pre-Check
		allowed, violation := s.node.Defederation.ValidatePost(req.Content, s.identity.Address, s.identity.PubKeyHex)
		if !allowed {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error":  violation,
				"policy": "Zero-tolerance anti-hate policy enforced",
			})
			return
		}

		powRes, err := crypto.Mine(req.Content, s.node.Difficulty)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "pow mining failed"})
			return
		}

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
	// Inject random hidden canary trap link into served HTML
	canaryPath := s.shield.GetRandomCanaryPath()
	htmlStr := strings.Replace(
		string(indexHTML),
		"</body>",
		`<a href="`+canaryPath+`" style="display:none!important;visibility:hidden;opacity:0;pointer-events:none;" aria-hidden="true" rel="nofollow"></a></body>`,
		1,
	)
	_, _ = w.Write([]byte(htmlStr))
}
