package storage

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

type Board struct {
	Slug        string    `json:"slug"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type Thread struct {
	ID            int64     `json:"id"`
	ThreadRef     string    `json:"thread_ref"`
	BoardSlug     string    `json:"board_slug"`
	Title         string    `json:"title"`
	AuthorAddress string    `json:"author_address"`
	PostCount     int       `json:"post_count"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

type Post struct {
	ID            int64     `json:"id"`
	ThreadID      int64     `json:"thread_id"`
	ThreadRef     string    `json:"thread_ref"`
	BoardSlug     string    `json:"board_slug"`
	Title         string    `json:"title,omitempty"` // populated if OP
	AuthorAddress string    `json:"author_address"`
	AuthorPubKey  string    `json:"author_pubkey"`
	Content       string    `json:"content"`
	Nonce         uint64    `json:"nonce"`
	PoWHash       string    `json:"pow_hash"`
	Signature     string    `json:"signature"`
	Timestamp     int64     `json:"timestamp"`
	IsOP          bool      `json:"is_op"`
}

type PeerRecord struct {
	Address   string    `json:"address"`
	Endpoint  string    `json:"endpoint"`
	LastSeen  time.Time `json:"last_seen"`
	Connected bool      `json:"connected"`
}

type DB struct {
	db *sql.DB
}

func Open(dbPath string) (*DB, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open sqlite database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;"); err != nil {
		return nil, fmt.Errorf("failed to configure sqlite PRAGMA: %w", err)
	}

	storage := &DB{db: db}
	if err := storage.initSchema(); err != nil {
		return nil, fmt.Errorf("schema initialization failed: %w", err)
	}

	return storage, nil
}

func (s *DB) Close() error {
	return s.db.Close()
}

func (s *DB) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS boards (
		slug TEXT PRIMARY KEY,
		title TEXT NOT NULL,
		description TEXT NOT NULL,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS threads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		thread_ref TEXT UNIQUE NOT NULL,
		board_slug TEXT NOT NULL,
		title TEXT NOT NULL,
		author_address TEXT NOT NULL,
		post_count INTEGER DEFAULT 1,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY(board_slug) REFERENCES boards(slug) ON DELETE CASCADE
	);

	CREATE TABLE IF NOT EXISTS posts (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		thread_id INTEGER NOT NULL,
		thread_ref TEXT NOT NULL,
		board_slug TEXT NOT NULL,
		author_address TEXT NOT NULL,
		author_pubkey TEXT NOT NULL,
		content TEXT NOT NULL,
		nonce INTEGER NOT NULL,
		pow_hash TEXT NOT NULL,
		signature TEXT NOT NULL,
		timestamp INTEGER NOT NULL,
		is_op BOOLEAN NOT NULL DEFAULT 0,
		FOREIGN KEY(thread_id) REFERENCES threads(id) ON DELETE CASCADE,
		FOREIGN KEY(board_slug) REFERENCES boards(slug) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_posts_thread ON posts(thread_id, id);
	CREATE INDEX IF NOT EXISTS idx_posts_ref ON posts(thread_ref);
	CREATE INDEX IF NOT EXISTS idx_threads_board_updated ON threads(board_slug, updated_at DESC);

	CREATE TABLE IF NOT EXISTS peers (
		address TEXT PRIMARY KEY,
		endpoint TEXT NOT NULL,
		last_seen TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		connected BOOLEAN DEFAULT 0
	);
	`
	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	defaultBoards := []Board{
		{Slug: "g", Title: "Technology & Networks", Description: "P2P architectures, cryptography, overlays, and decentralized protocols."},
		{Slug: "crypto", Title: "Cryptography & Privacy", Description: "Zero-knowledge proofs, onion routing, self-sovereign keys, and cipher design."},
		{Slug: "freedom", Title: "Free Speech & Open Web", Description: "Censorship resistance, digital rights, and sovereign human communication."},
		{Slug: "b", Title: "Random", Description: "Open discussion, memes, and unstructured peer-to-peer discourse."},
	}

	for _, b := range defaultBoards {
		s.db.Exec(`INSERT OR IGNORE INTO boards (slug, title, description) VALUES (?, ?, ?)`, b.Slug, b.Title, b.Description)
	}

	return nil
}

// CreatePost handles thread creation or resolution via decentralized ThreadRef.
func (s *DB) CreatePost(p *Post) (postID int64, threadID int64, err error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, err
	}
	defer tx.Rollback()

	// If ThreadRef is empty but ThreadID is given, resolve ThreadRef from existing thread
	if p.ThreadRef == "" && p.ThreadID != 0 {
		_ = tx.QueryRow(`SELECT thread_ref FROM threads WHERE id = ?`, p.ThreadID).Scan(&p.ThreadRef)
	}

	// If ThreadRef is still empty, derive from PoWHash or Signature
	if p.ThreadRef == "" {
		if p.PoWHash != "" {
			p.ThreadRef = p.PoWHash
		} else {
			p.ThreadRef = fmt.Sprintf("ref:%d:%s", p.Timestamp, p.AuthorAddress)
		}
	}

	var existingThreadID int64
	err = tx.QueryRow(`SELECT id FROM threads WHERE thread_ref = ?`, p.ThreadRef).Scan(&existingThreadID)
	if err == sql.ErrNoRows {
		// Thread does not exist yet: Create new Thread
		title := p.Title
		if title == "" {
			title = "Untitled Discussion"
		}
		res, err := tx.Exec(`
			INSERT INTO threads (thread_ref, board_slug, title, author_address, post_count, updated_at)
			VALUES (?, ?, ?, ?, 1, CURRENT_TIMESTAMP)
		`, p.ThreadRef, p.BoardSlug, title, p.AuthorAddress)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to create thread: %w", err)
		}

		tID, err := res.LastInsertId()
		if err != nil {
			return 0, 0, err
		}
		p.ThreadID = tID
		p.IsOP = true
	} else if err != nil {
		return 0, 0, fmt.Errorf("failed to check existing thread: %w", err)
	} else {
		// Existing thread: Bump post count and timestamp
		p.ThreadID = existingThreadID
		p.IsOP = false
		_, err := tx.Exec(`
			UPDATE threads 
			SET post_count = post_count + 1, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?
		`, p.ThreadID)
		if err != nil {
			return 0, 0, fmt.Errorf("failed to bump thread: %w", err)
		}
	}

	res, err := tx.Exec(`
		INSERT INTO posts (thread_id, thread_ref, board_slug, author_address, author_pubkey, content, nonce, pow_hash, signature, timestamp, is_op)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, p.ThreadID, p.ThreadRef, p.BoardSlug, p.AuthorAddress, p.AuthorPubKey, p.Content, p.Nonce, p.PoWHash, p.Signature, p.Timestamp, p.IsOP)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to insert post: %w", err)
	}

	pID, err := res.LastInsertId()
	if err != nil {
		return 0, 0, err
	}

	if err := tx.Commit(); err != nil {
		return 0, 0, err
	}

	return pID, p.ThreadID, nil
}

func (s *DB) GetBoards() ([]Board, error) {
	rows, err := s.db.Query(`SELECT slug, title, description, created_at FROM boards ORDER BY slug ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var boards []Board
	for rows.Next() {
		var b Board
		if err := rows.Scan(&b.Slug, &b.Title, &b.Description, &b.CreatedAt); err != nil {
			return nil, err
		}
		boards = append(boards, b)
	}
	return boards, nil
}

func (s *DB) GetThreads(boardSlug string, limit int) ([]Thread, error) {
	rows, err := s.db.Query(`
		SELECT id, thread_ref, board_slug, title, author_address, post_count, created_at, updated_at
		FROM threads 
		WHERE board_slug = ? 
		ORDER BY updated_at DESC 
		LIMIT ?
	`, boardSlug, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var threads []Thread
	for rows.Next() {
		var t Thread
		if err := rows.Scan(&t.ID, &t.ThreadRef, &t.BoardSlug, &t.Title, &t.AuthorAddress, &t.PostCount, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, err
		}
		threads = append(threads, t)
	}
	return threads, nil
}

func (s *DB) GetPosts(threadID int64) ([]Post, error) {
	rows, err := s.db.Query(`
		SELECT id, thread_id, thread_ref, board_slug, author_address, author_pubkey, content, nonce, pow_hash, signature, timestamp, is_op
		FROM posts 
		WHERE thread_id = ? 
		ORDER BY id ASC
	`, threadID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var posts []Post
	for rows.Next() {
		var p Post
		if err := rows.Scan(&p.ID, &p.ThreadID, &p.ThreadRef, &p.BoardSlug, &p.AuthorAddress, &p.AuthorPubKey, &p.Content, &p.Nonce, &p.PoWHash, &p.Signature, &p.Timestamp, &p.IsOP); err != nil {
			return nil, err
		}
		posts = append(posts, p)
	}
	return posts, nil
}

func (s *DB) UpsertPeer(address, endpoint string, connected bool) error {
	_, err := s.db.Exec(`
		INSERT INTO peers (address, endpoint, last_seen, connected)
		VALUES (?, ?, CURRENT_TIMESTAMP, ?)
		ON CONFLICT(address) DO UPDATE SET
			endpoint = excluded.endpoint,
			last_seen = CURRENT_TIMESTAMP,
			connected = excluded.connected
	`, address, endpoint, connected)
	return err
}

func (s *DB) GetPeers() ([]PeerRecord, error) {
	rows, err := s.db.Query(`SELECT address, endpoint, last_seen, connected FROM peers ORDER BY last_seen DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var peers []PeerRecord
	for rows.Next() {
		var p PeerRecord
		if err := rows.Scan(&p.Address, &p.Endpoint, &p.LastSeen, &p.Connected); err != nil {
			return nil, err
		}
		peers = append(peers, p)
	}
	return peers, nil
}
