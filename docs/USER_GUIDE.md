# Sovereign Mesh User & Operator Guide 📖

This guide provides step-by-step instructions for running a **Sovereign Mesh** node, interacting with the peer-to-peer overlay, posting to decentralized imageboards, and configuring multi-node networks.

---

## Table of Contents
1. [System Requirements & Installation](#1-system-requirements--installation)
2. [Identity Management (`keygen`)](#2-identity-management-keygen)
3. [Running an Overlay Node (`node`)](#3-running-an-overlay-node-node)
4. [Using the Web Imageboard Interface](#4-using-the-web-imageboard-interface)
5. [Posting via Command-Line Interface (`post`)](#5-posting-via-command-line-interface-post)
6. [Multi-Node Peering & Network Topology](#6-multi-node-peering--network-topology)
7. [Benchmarking Proof-of-Work (`bench`)](#7-benchmarking-proof-of-work-bench)
8. [Anti-Scraping & Crawler Defense Guidelines](#8-anti-scraping--crawler-defense-guidelines)

---

## 1. System Requirements & Installation

### Requirements
- **Operating System:** Linux, macOS, or Windows
- **Go Compiler:** Go 1.22 or higher (pure Go, zero CGO dependencies)
- **Memory:** Minimum 128 MB RAM (Argon2id mining allocates 32 MB per active hash cycle)
- **Disk Space:** ~20 MB for binary + storage for local SQLite database

### Building from Source
```bash
# Clone the repository
git clone https://github.com/ajejfiejof/sovereign-mesh.git
cd sovereign-mesh

# Build with stripped binary symbols using Make
make build

# Or compile directly with standard Go:
CGO_ENABLED=0 go build -ldflags="-s -w" -o sovereign-mesh ./cmd/sovereign-mesh
```

---

## 2. Identity Management (`keygen`)

Sovereign Mesh does not use user accounts, passwords, emails, or phone numbers. Your identity is a self-sovereign **Ed25519 cryptographic keypair**.

### Generate a Keypair
```bash
./sovereign-mesh keygen
```
**Example Output:**
```
Cryptographic Address:    mesh:d32a5767882ed68e2d505e956a6879c0440aa238
Public Key (Ed25519 Hex): 31016bb28f43b869a42eae84ae0a44d966989246355bfd135d425dffc06cbcbd
Private Key (Secret Hex): 72e5a212388d24b642a48581d3d4b5b604a83010525f60f0b67e1b436e0277b8...
```

### Save Keypair to a File
```bash
./sovereign-mesh keygen -out my_identity.json
```

---

## 3. Running an Overlay Node (`node`)

Launching the node starts both the background **P2P TCP listener** and the **local Web Imageboard portal**:

```bash
# Start node on default ports (P2P: 9000, Web: 8080)
./sovereign-mesh node
```

### Command Flags
| Flag | Default | Description |
|:---|:---:|:---|
| `-p2p` | `9000` | TCP port for the P2P overlay gossip listener |
| `-web` | `8080` | HTTP port for the web imageboard interface |
| `-diff` | `2` | Argon2id PoW difficulty (number of leading hex zeros required) |
| `-data` | `~/.sovereign-mesh` | Path to storage directory (holds `mesh.db` and `identity.json`) |
| `-peer` | `""` | Optional initial bootstrap peer endpoint (`host:port`) |

### Example: Running a Custom Node
```bash
./sovereign-mesh node -p2p 9100 -web 8100 -diff 2 -data ./node_storage
```

---

## 4. Using the Web Imageboard Interface

1. Start your node: `./sovereign-mesh node`
2. Open your web browser to **`http://127.0.0.1:8080`**
3. The interface will load the default boards:
   - **/g/** — Technology, P2P Networks, Overlays
   - **/crypto/** — Cryptography, Privacy, Onion Routing
   - **/freedom/** — Free Speech, Decentralization, Civil Rights
   - **/b/** — Random, Unstructured Discourse

### Creating a New Thread
1. Navigate to your desired board (e.g. `/g/`).
2. Enter an optional **Thread Subject / Title**.
3. Type your post message into the content box.
4. Click **"MINE POW & PUBLISH OVERLAY POST"**.
5. Your browser sends the post to your local node, which computes the Argon2id memory-hard Proof-of-Work, signs the content with your private key, saves it locally, and gossips it to all connected peers.

### Replying to a Thread
Threads automatically bump to the top of the board upon new replies. To reply, specify the thread reference ID or reply from the thread view.

---

## 5. Posting via Command-Line Interface (`post`)

You can create threads and submit replies directly from your shell without opening a browser:

```bash
# Create a new thread on board /g/
./sovereign-mesh post -board g -title "P2P Scalability" -content "Testing CLI-based decentralized gossip."

# Post a reply to an existing thread (e.g., Thread ID 1)
./sovereign-mesh post -board g -thread 1 -content "Replying directly from the terminal."
```

---

## 6. Multi-Node Peering & Network Topology

To form a decentralized network, nodes connect over TCP and mutually authenticate:

### Local Two-Node Setup
```bash
# Terminal 1: Launch Node A
./sovereign-mesh node -p2p 9000 -web 8080 -data ./node_a

# Terminal 2: Launch Node B and peer with Node A
./sovereign-mesh node -p2p 9001 -web 8081 -data ./node_b -peer 127.0.0.1:9000
```

### Inspect Connected Peers
```bash
# Query active peers from Node A
./sovereign-mesh peers -port 8080

# Query active peers from Node B
./sovereign-mesh peers -port 8081
```

### Automatic Peer Exchange (PEX)
When Node C connects to Node B, Node B automatically informs Node C of Node A's address via the **Peer Exchange Protocol (PEX)**. Posts published on Node C will automatically route across Node B and appear on Node A.

---

## 7. Benchmarking Proof-of-Work (`bench`)

Test your local CPU's mining speed across different Argon2id difficulty targets (32 MB RAM memory challenge):

```bash
./sovereign-mesh bench
```

**Output:**
```
Benchmarking Argon2id Memory-Hard Proof-of-Work (32 MB RAM per hash attempt):
Mining Difficulty 1 (prefix '0')...
  -> Solved in 1.35s! Nonce: 45 | Hash: 063069528fb3... | Speed: 33.9 H/s
Mining Difficulty 2 (prefix '00')...
  -> Solved in 14.1s! Nonce: 422 | Hash: 009f3cc0ffec... | Speed: 30.0 H/s
```

---

## 8. Anti-Scraping & Crawler Defense Guidelines

Sovereign Mesh includes built-in anti-scraping protections (`pkg/antiscrape`):

1. **User-Agent Policy:** Standard scraping libraries (`python-requests`, `urllib`, `curl`, `Scrapy`) are automatically tarpitted. If you are developing custom tooling for your own node, set a custom User-Agent starting with `SovereignMesh-` or use a standard browser header.
2. **Canary Honeypot Links:** The web interface injects hidden `<a href="/api/canary/*">` links. If an automated script crawls these links, the client IP is banned for 1 hour.
3. **Read Rate-Limiting:** If your script reads more than 30 requests/minute, the server responds with `HTTP 429 Too Many Requests` and an `X-Mesh-PoW-Challenge` header.
