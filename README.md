# Sovereign Mesh (`sovereign-mesh`) 🌐
> A self-sovereign, cryptographically addressed P2P communication overlay and local-first imageboard daemon with Argon2id anti-spam Proof-of-Work and multi-tier anti-scraping defenses in pure Go.

[![License: AGPL v3](https://img.shields.io/badge/License-AGPLv3-blue.svg)](LICENSE)
[![Go Report Card](https://goreportcard.com/badge/github.com/ajejfiejof/sovereign-mesh)](https://goreportcard.com/report/github.com/ajejfiejof/sovereign-mesh)
[![Test Coverage: 77%](https://img.shields.io/badge/Coverage-77%25-brightgreen.svg)](coverage.out)

---

## The Problem It Solves

As debated on technical forums, modern internet communication suffers from structural centralisation:
1. **IPv4 & DNS Gatekeeping:** ISPs assign IP addresses and ICANN controls domains, forcing self-hosters through Cloudflare, certificate authorities, and corporate hyperscalers.
2. **The Darknet Usability Paradox:** Traditional darknets (Tor, I2P) suffer from high configuration barriers, TCP latency, and dead links when single host servers go offline.
3. **The Bot Flood & Scraper Problem:** Unpermissioned networks are easily drowned in automated bot spam, volumetric DDoS attacks, and aggressive scraper crawling.

**Sovereign Mesh** solves these problems by combining:
- **Cryptographic Node Addressing:** Your network address is derived from an Ed25519 public key (`mesh:<hex>`). No DNS, no ISP assignment, no domain registration.
- **Argon2id Memory-Hard Anti-Spam:** Every post requires 1–2 seconds of local CPU memory-hard computation (32 MB RAM challenge). Bot farms attempting 100,000 posts/minute are rendered economically unviable.
- **Multi-Tier Anti-Scraping Defense Suite:** Built-in honeypot canary links, token-bucket read rate-limiting with PoW challenges, HTTP tarpitting for detected crawlers, and invisible zero-width Unicode text watermarking.
- **Local-First Replicated Data:** Threads are stored in an embedded SQLite database and gossiped peer-to-peer. A thread stays online as long as *any* peer in the network has read it.
- **Turnkey Single Binary:** Compiles with zero CGO dependencies into a standalone executable that serves both the P2P overlay and an embedded web imageboard at `http://127.0.0.1:8080`.

---

## Anti-Scraping Defense Suite (`pkg/antiscrape`)

Sovereign Mesh includes an active anti-scraping and crawler neutralization engine:

| Defense Layer | Mechanism | Target Threat |
|:---|:---|:---|
| **Canary Honeypots** | Injects hidden, invisible links into served HTML DOM (`/api/canary/*`). | Automated scrapers crawling all DOM links trigger instant 1-hour IP bans. |
| **HTTP Tarpitting** | Flagged bot connections receive trickled responses (1 byte/sec). | Exhausts crawler socket pools and hangs scraper concurrency. |
| **Agent Heuristics** | Detects `python-requests`, `aiohttp`, `curl`, `Scrapy`, `urllib`, etc. | Automatically blocks programmatic scraping libraries. |
| **Read Rate-Limiting** | Token-bucket sliding window with `X-Mesh-PoW-Challenge`. | Prevents bulk database scraping without computational cost. |
| **Text Watermarking** | Embeds invisible zero-width Unicode (`\u200B`, `\u200C`) into served text. | Watermarks stolen content and breaks automated regex scrapers. |

---

## Documentation

- 📖 **[User & Operator Guide](docs/USER_GUIDE.md):** Complete guide to identity management, node deployment, multi-node peering, and CLI posting.
- 📡 **[REST API Reference](docs/API_REFERENCE.md):** Comprehensive endpoint reference with curl examples, headers, and request/response payloads.
- ⚙️ **[Wire Protocol Specification](docs/PROTOCOL.md):** Low-level packet framing, binary opcodes, mutual Ed25519 handshakes, and Argon2id parameters.

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
 │  [Anti-Scrape Shield] ◄── Canary Honeypot + Tarpit Engine   │
 │       │                                                     │
 │  [Spam Filter]        ◄── Argon2id PoW (Memory-hard 32MB)   │
 │       │                                                     │
 │  [Storage Engine]     ◄── Pure Go SQLite Engine (WAL mode)  │
 │       │                   (Threads replicated via ThreadRef)│
 │       │                                                     │
 │  [Gossip Engine]      ◄── SovereignMesh Wire Protocol (TCP) │
 │       │                   (Mutual Ed25519 Handshake + PEX)  │
 │       │                                                     │
 │  [Identity]           ◄── Ed25519 Keypair (mesh:<sha256>)   │
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
make build
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

## Test Coverage

Every core component has dedicated unit and integration tests:

| Package | Statement Coverage | Test Scope |
|:---|:---:|:---|
| **`pkg/crypto`** | **94.4%** | Ed25519 identity generation, file I/O, hex codecs, Argon2id mining & verification |
| **`pkg/antiscrape`** | **87.1%** | User-agent detection, canary honeypots, token bucket rate limiting & tarpitting |
| **`pkg/storage`** | **78.0%** | SQLite schema initialization, atomic thread creation, bumping, and ephemeral pruning |
| **`cmd/sovereign-mesh`** | **75.2%** | CLI keygen, bench, usage, post, peers, and background node lifecycle |
| **`pkg/web`** | **73.9%** | REST API endpoints, HTML delivery, client post ingestion & anti-scrape integration |
| **`pkg/p2p`** | **71.6%** | Wire protocol framing, PEX discovery, mutual handshakes & gossip replication |
| **Total Project** | **77.3%** | Tested across 5 Go packages |

To run the full test and coverage suite:
```bash
make coverage
```

---

## License

This project is licensed under the **GNU Affero General Public License v3.0 (AGPLv3)**. See the [LICENSE](LICENSE) file for the full license text.
