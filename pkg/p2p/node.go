package p2p

import (
	"context"
	"fmt"
	"log"
	"net"
	"strconv"
	"sync"
	"time"

	"github.com/ajejfiejof/sovereign-mesh/pkg/crypto"
	"github.com/ajejfiejof/sovereign-mesh/pkg/moderation"
	"github.com/ajejfiejof/sovereign-mesh/pkg/storage"
)

type Node struct {
	Identity     *crypto.Identity
	Store        *storage.DB
	Defederation *moderation.DefederationManager
	Port         int
	Difficulty   int
	peers        map[string]*Peer
	peersMu      sync.RWMutex
	seenPosts    map[string]bool
	seenMu       sync.RWMutex
	listener     net.Listener
}

func NewNode(id *crypto.Identity, store *storage.DB, port int, difficulty int) *Node {
	return &Node{
		Identity:     id,
		Store:        store,
		Defederation: moderation.NewDefederationManager(),
		Port:         port,
		Difficulty:   difficulty,
		peers:        make(map[string]*Peer),
		seenPosts:    make(map[string]bool),
	}
}

func (n *Node) Start(ctx context.Context) error {
	addr := net.JoinHostPort("0.0.0.0", strconv.Itoa(n.Port))
	l, err := net.Listen("tcp", addr)
	if err != nil {
		return fmt.Errorf("failed to bind P2P listener on %s: %w", addr, err)
	}
	n.listener = l
	log.Printf("[P2P] Node online at %s (Address: %s, PoW Difficulty: %d)", addr, n.Identity.Address, n.Difficulty)

	go func() {
		<-ctx.Done()
		_ = n.listener.Close()
	}()

	go n.acceptLoop()
	go n.keepaliveLoop(ctx)
	go n.maintenanceLoop(ctx)

	return nil
}

func (n *Node) acceptLoop() {
	for {
		conn, err := n.listener.Accept()
		if err != nil {
			return
		}
		go n.handleInboundConn(conn)
	}
}

func (n *Node) handleInboundConn(conn net.Conn) {
	peer := NewPeer(conn, conn.RemoteAddr().String(), n)
	peer.Start()

	// Send our handshake
	n.sendHandshake(peer)
}

func (n *Node) ConnectPeer(endpoint string) error {
	conn, err := net.DialTimeout("tcp", endpoint, 5*time.Second)
	if err != nil {
		return fmt.Errorf("connection to %s failed: %w", endpoint, err)
	}

	peer := NewPeer(conn, endpoint, n)
	peer.Start()

	// Send our handshake
	n.sendHandshake(peer)
	return nil
}

func (n *Node) sendHandshake(p *Peer) {
	now := time.Now().Unix()
	msgBytes := []byte(fmt.Sprintf("%s:%d", n.Identity.Address, now))
	sig := n.Identity.Sign(msgBytes)

	hs := HandshakeMessage{
		Address:   n.Identity.Address,
		PubKey:    n.Identity.PubKeyHex,
		Timestamp: now,
		Signature: sig,
	}

	payload, err := EncodeJSON(hs)
	if err == nil {
		p.Send(&Packet{Type: MsgTypeHandshake, Payload: payload})
	}
}

func (n *Node) sendPeerExchange(p *Peer) {
	records, err := n.Store.GetPeers()
	if err != nil || len(records) == 0 {
		return
	}

	var endpoints []string
	for _, r := range records {
		if r.Endpoint != "" && r.Endpoint != p.Endpoint {
			endpoints = append(endpoints, r.Endpoint)
		}
	}

	if len(endpoints) > 0 {
		payload, err := EncodeJSON(PeerExchangeMessage{Endpoints: endpoints})
		if err == nil {
			p.Send(&Packet{Type: MsgTypePeerExchange, Payload: payload})
		}
	}
}

func (n *Node) handlePacket(p *Peer, pkt *Packet) {
	switch pkt.Type {
	case MsgTypeHandshake:
		var hs HandshakeMessage
		if err := DecodeJSON(pkt.Payload, &hs); err != nil {
			p.Close()
			return
		}

		msgBytes := []byte(fmt.Sprintf("%s:%d", hs.Address, hs.Timestamp))
		if !crypto.VerifySignature(hs.PubKey, hs.Signature, msgBytes) {
			log.Printf("[P2P] Rejected invalid handshake signature from %s", p.Endpoint)
			p.Close()
			return
		}

		p.Address = hs.Address
		n.addPeer(p)
		_ = n.Store.UpsertPeer(p.Address, p.Endpoint, true)

		// Send Ack
		ackPayload, _ := EncodeJSON(map[string]string{"status": "ok", "address": n.Identity.Address})
		p.Send(&Packet{Type: MsgTypeHandshakeAck, Payload: ackPayload})

		// Send PEX peer exchange list
		n.sendPeerExchange(p)

	case MsgTypeHandshakeAck:
		log.Printf("[P2P] Mutual handshake established with %s", p.Endpoint)
		n.sendPeerExchange(p)

	case MsgTypePeerExchange:
		var pex PeerExchangeMessage
		if err := DecodeJSON(pkt.Payload, &pex); err == nil {
			for _, ep := range pex.Endpoints {
				_ = n.Store.UpsertPeer("discovered:"+ep, ep, false)
			}
		}

	case MsgTypeGossipPost:
		n.handleGossipPost(p, pkt.Payload)

	case MsgTypePing:
		p.Send(&Packet{Type: MsgTypePong, Payload: []byte("pong")})

	case MsgTypePong:
		// Heartbeat acknowledged
	}
}

func (n *Node) handleGossipPost(sender *Peer, payload []byte) {
	var post storage.Post
	if err := DecodeJSON(payload, &post); err != nil {
		return
	}

	postIDKey := fmt.Sprintf("%s:%d", post.AuthorAddress, post.Timestamp)

	n.seenMu.Lock()
	if n.seenPosts[postIDKey] {
		n.seenMu.Unlock()
		return
	}
	n.seenPosts[postIDKey] = true
	n.seenMu.Unlock()

	// Zero-Tolerance Inbound Moderation & Defederation Check
	allowed, violation := n.Defederation.ValidatePost(post.Content, post.AuthorAddress, post.AuthorPubKey)
	if !allowed {
		log.Printf("[P2P-Moderation] ZERO TOLERANCE: Dropping post and defederating peer %s (%s)", sender.Endpoint, violation)
		n.removePeer(sender.Address)
		sender.Close()
		return
	}

	if !crypto.VerifySignature(post.AuthorPubKey, post.Signature, []byte(post.Content)) {
		log.Printf("[P2P] Rejected gossip post: invalid signature from %s", post.AuthorAddress)
		return
	}

	if !crypto.VerifyPoW(post.Content, post.Nonce, n.Difficulty) {
		log.Printf("[P2P] Rejected gossip post: insufficient PoW from %s", post.AuthorAddress)
		return
	}

	_, _, err := n.Store.CreatePost(&post)
	if err != nil {
		log.Printf("[P2P] Failed to store gossip post locally: %v", err)
		return
	}

	log.Printf("[P2P] Ingested verified post from %s on board /%s/ (Thread %d)", post.AuthorAddress, post.BoardSlug, post.ThreadID)
	n.BroadcastPost(&post, sender.Address)
}

func (n *Node) BroadcastPost(post *storage.Post, excludeAddress ...string) {
	payload, err := EncodeJSON(post)
	if err != nil {
		return
	}

	exclude := ""
	if len(excludeAddress) > 0 {
		exclude = excludeAddress[0]
	}

	pkt := &Packet{Type: MsgTypeGossipPost, Payload: payload}

	n.peersMu.RLock()
	defer n.peersMu.RUnlock()

	for addr, peer := range n.peers {
		if addr != exclude {
			peer.Send(pkt)
		}
	}
}

func (n *Node) addPeer(p *Peer) {
	n.peersMu.Lock()
	defer n.peersMu.Unlock()
	n.peers[p.Address] = p
	log.Printf("[P2P] Connected to peer %s (%s)", p.Address, p.Endpoint)
}

func (n *Node) removePeer(address string) {
	n.peersMu.Lock()
	defer n.peersMu.Unlock()
	delete(n.peers, address)
	_ = n.Store.UpsertPeer(address, "", false)
}

func (n *Node) GetActivePeers() []string {
	n.peersMu.RLock()
	defer n.peersMu.RUnlock()
	var list []string
	for addr := range n.peers {
		list = append(list, addr)
	}
	return list
}

func (n *Node) PruneAllBoards(maxKeep int) {
	boards, err := n.Store.GetBoards()
	if err != nil {
		return
	}
	for _, b := range boards {
		_, _ = n.Store.PruneThreads(b.Slug, maxKeep)
	}
}

func (n *Node) keepaliveLoop(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pingPkt := &Packet{Type: MsgTypePing, Payload: []byte("ping")}
			n.peersMu.RLock()
			for _, p := range n.peers {
				p.Send(pingPkt)
			}
			n.peersMu.RUnlock()
		}
	}
}

func (n *Node) maintenanceLoop(ctx context.Context) {
	ticker := time.NewTicker(15 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			n.PruneAllBoards(50)
		}
	}
}
