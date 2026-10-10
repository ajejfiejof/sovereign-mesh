package p2p

import (
	"net"
	"sync"
	"time"
)

type Peer struct {
	Address    string
	Endpoint   string
	Conn       net.Conn
	Node       *Node
	sendQueue  chan *Packet
	closeOnce  sync.Once
	isClosed   bool
	mu         sync.Mutex
	lastActive time.Time
}

func NewPeer(conn net.Conn, endpoint string, node *Node) *Peer {
	return &Peer{
		Endpoint:   endpoint,
		Conn:       conn,
		Node:       node,
		sendQueue:  make(chan *Packet, 64),
		lastActive: time.Now(),
	}
}

func (p *Peer) Start() {
	go p.writeLoop()
	go p.readLoop()
}

func (p *Peer) Close() {
	p.closeOnce.Do(func() {
		p.mu.Lock()
		p.isClosed = true
		p.mu.Unlock()
		close(p.sendQueue)
		_ = p.Conn.Close()
		p.Node.removePeer(p.Address)
	})
}

func (p *Peer) Send(pkt *Packet) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.isClosed {
		return
	}
	select {
	case p.sendQueue <- pkt:
	default:
		// Drop packet if peer queue is backed up
	}
}

func (p *Peer) writeLoop() {
	for pkt := range p.sendQueue {
		_ = p.Conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
		if err := WritePacket(p.Conn, pkt.Type, pkt.Payload); err != nil {
			p.Close()
			return
		}
	}
}

func (p *Peer) readLoop() {
	defer p.Close()
	for {
		_ = p.Conn.SetReadDeadline(time.Now().Add(90 * time.Second))
		pkt, err := ReadPacket(p.Conn)
		if err != nil {
			return
		}
		p.lastActive = time.Now()
		p.Node.handlePacket(p, pkt)
	}
}
