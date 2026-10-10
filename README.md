# Sovereign Mesh (`sovereign-mesh`) 🌐
> A self-sovereign, cryptographically addressed P2P communication overlay and local-first imageboard daemon with Argon2id anti-spam Proof-of-Work in pure Go.

[![License: AGPL v3](https://img.shields.io/badge/License-AGPLv3-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ajejfiejof/sovereign-mesh)](https://goreportcard.com/report/github.com/ajejfiejof/sovereign-mesh)
[![Test Coverage: 69%](https://img.shields.io/badge/Coverage-69%25-brightgreen.svg)](coverage.out)

---

## The Problem It Solves

As debated on technical forums, modern internet communication suffers from structural centralisation:
1. **IPv4 & DNS Gatekeeping:** ISPs assign IP addresses and ICANN controls domains, forcing self-hosters through Cloudflare, certificate authorities, and corporate hyperscalers.
2. **The Darknet Usability Paradox:** Traditional darknets (Tor, I2P) suffer from high configuration barriers, TCP latency, and dead links when single host servers go offline.
3. **The Bot Flood Problem:** Unpermissioned networks are easily drowned in automated bot spam and volumetric DDoS attacks.

**Sovereign Mesh** solves these problems by combining:
- **Cryptographic Node Addressing:** Your network address is derived from an Ed25519 public key (`mesh:<hex>`). No DNS, no ISP assignment, no domain registration.
- **Argon2id Memory-Hard Anti-Spam:** Every post requires 1–2 seconds of local CPU memory-hard computation (32 MB RAM challenge). Bot farms attempting 100,000 posts/minute are rendered economically unviable, while remaining transparent to real humans.
- **Local-First Replicated Data:** Threads are stored in an embedded SQLite database and gossiped peer-to-peer. A thread stays online as long as *any* peer in the network has read it.
- **Turnkey Single Binary:** Compiles with zero CGO dependencies into a standalone executable that serves both the P2P overlay and an embedded web imageboard at `http://127.0.0.1:8080`.

---

## Architecture

```
                       [ USER WEB BROWSER ]
                   (http://127.0.0.1:8080)
                              │
                      HTTP / REST API
                              │
               [ SOVEREIGN MESH DAEMON (Go) ]
 ┌─────────────────────────────────────────────────────────────┐
 │                                                             │
 │  [Spam Filter]  ◄── Argon2id PoW (Memory-hard difficulty)   │
 │       │                                                     │
 │  [Storage]      ◄── Pure Go SQLite Engine (WAL mode)        │
 │       │             (Threads replicated via ThreadRef)      │
 │       │                                                     │
 │  [Gossip Engine]◄── SovereignMesh Wire Protocol (TCP)       │
 │       │             (Mutual Ed25519 Challenge Handshake)    │
 │       │                                                     │
 │  [Identity]     ◄── Ed25519 Keypair (mesh:<sha256(pub)[:20]>)│
 └─────────────────────────────────────────────────────────────┘
                              │
                    TCP Overlay Transport
                              │
                     [ NETWORK PEERS ]
```

---

## Quickstart

### 1. Build from Source
Requires Go 1.22+:
```bash
git clone https://github.com/ajejfiejof/sovereign-mesh.git
cd sovereign-mesh
go build -o sovereign-mesh ./cmd/sovereign-mesh
```

### 2. Generate a Cryptographic Identity
Generate a fresh self-sovereign identity keypair:
```bash
./sovereign-mesh keygen
```

### 3. Start an Overlay Node
Launch the node daemon with the embedded web imageboard portal:
```bash
# Node 1 on default ports (P2P: 9000, Web: 8080)
./sovereign-mesh node

# Connect to another peer on the network:
./sovereign-mesh node --p2p 9001 --web 8081 --peer 127.0.0.1:9000
```
Open **`http://127.0.0.1:8080`** in any web browser to browse `/g/`, `/crypto/`, `/freedom/`, and `/b/`, create threads, and reply.

### 4. Post via Command-Line
You can also mine and publish posts directly from the terminal:
```bash
./sovereign-mesh post --board g --title "Decentralization" --content "Hello from the overlay."
```

### 5. Benchmark Mining Speed
Benchmark your CPU's Argon2id memory-hard hashing throughput:
```bash
./sovereign-mesh bench
```

---

## REST API Reference

The embedded node exposes a local REST API:

| Endpoint | Method | Description |
|:---|:---:|:---|
| `/api/node/info` | `GET` | Node address, public key, active peers, and PoW difficulty |
| `/api/boards` | `GET` | List all boards (`/g/`, `/crypto/`, `/freedom/`, `/b/`) |
| `/api/threads?board=g` | `GET` | List active threads on a board |
| `/api/posts?thread_id=1` | `GET` | Fetch all posts in a thread |
| `/api/posts` | `POST` | Mine PoW, sign, persist, and gossip a new post |
| `/api/peers` | `GET` | List discovered peer routing records |
| `/api/peers/connect` | `POST` | Connect to an outbound peer endpoint (`{"endpoint": "host:port"}`) |

---

## Test Coverage

Every core component has dedicated unit and integration tests:

| Package | Statement Coverage | Test Scope |
|:---|:---:|:---|
| **`pkg/crypto`** | **94.4%** | Ed25519 identity generation, file I/O, hex codecs, Argon2id mining & verification |
| **`pkg/storage`** | **78.6%** | SQLite schema initialization, atomic thread creation, bumping, and peer tracking |
| **`pkg/p2p`** | **73.3%** | Wire protocol framing, multi-node peering, mutual handshakes & gossip replication |
| **`pkg/web`** | **73.3%** | REST API endpoints, HTML delivery, client post ingestion & CORS headers |
| **`cmd/sovereign-mesh`** | **43.8%** | CLI keygen, bench, usage, and end-to-end command execution |
| **Total Project** | **68.8%** | Tested across 5 Go packages |

To run the full test and coverage suite:
```bash
go test -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

---

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPLv3)**. See the [LICENSE](LICENSE) file for the full license text.
