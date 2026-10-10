# Sovereign Mesh REST API Reference 📡

The Sovereign Mesh daemon embeds a local HTTP server providing a RESTful API for reading boards, creating threads, fetching posts, and managing network peering.

**Base URL:** `http://127.0.0.1:8080` (or configured via `-web <port>`)

---

## Headers & Client Conventions

- **Content-Type:** `application/json` (required for `POST` endpoints).
- **User-Agent:** Generic scraping signatures (`python-requests`, `curl`, `Scrapy`) are challenged by the anti-scraping shield. Legitimate API integrations should pass a custom identifier (e.g. `User-Agent: SovereignMesh-Client/1.0` or standard browser header).
- **CORS:** Enabled by default (`Access-Control-Allow-Origin: *`) for local browser integrations.

---

## Endpoints Summary

| Endpoint | Method | Description |
|:---|:---:|:---|
| [`/api/node/info`](#1-get-apinodeinfo) | `GET` | Get node status, cryptographic address, and peer stats |
| [`/api/boards`](#2-get-apiboards) | `GET` | List all available message boards |
| [`/api/threads`](#3-get-apithreads) | `GET` | List active threads on a specific board |
| [`/api/posts`](#4-get-apiposts) | `GET` | Retrieve all posts within a specific thread |
| [`/api/posts`](#5-post-apiposts) | `POST` | Mine PoW, sign, and publish a new thread or reply |
| [`/api/peers`](#6-get-apipeers) | `GET` | List discovered network peers and routing status |
| [`/api/peers/connect`](#7-post-apipeersconnect) | `POST` | Manually connect to an outbound P2P peer |

---

## Detailed Endpoint Documentation

### 1. `GET /api/node/info`
Returns metadata regarding the local node's cryptographic identity, network listener, active peers, and current mining difficulty.

#### Example Request:
```bash
curl -H "User-Agent: SovereignMesh-Client/1.0" \
     http://127.0.0.1:8080/api/node/info
```

#### Example Response (HTTP 200):
```json
{
  "active_peers": 2,
  "address": "mesh:455812a1763303dc542331ee39368d04100a5867",
  "antiscrape": "active",
  "difficulty": 2,
  "p2p_port": 9000,
  "peer_list": [
    "mesh:1d344b04960914f8e0bd5c9632ba555d8d4862aa",
    "mesh:874e19c800033efd31c73159c01f568a547ec5a1"
  ],
  "pubkey": "31016bb28f43b869a42eae84ae0a44d966989246355bfd135d425dffc06cbcbd",
  "status": "online"
}
```

---

### 2. `GET /api/boards`
Returns a list of all boards configured in the local database.

#### Example Request:
```bash
curl -H "User-Agent: SovereignMesh-Client/1.0" \
     http://127.0.0.1:8080/api/boards
```

#### Example Response (HTTP 200):
```json
[
  {
    "slug": "b",
    "title": "Random",
    "description": "Open discussion, memes, and unstructured peer-to-peer discourse.",
    "created_at": "2026-10-10T00:00:00Z"
  },
  {
    "slug": "crypto",
    "title": "Cryptography & Privacy",
    "description": "Zero-knowledge proofs, onion routing, self-sovereign keys, and cipher design.",
    "created_at": "2026-10-10T00:00:00Z"
  },
  {
    "slug": "freedom",
    "title": "Free Speech & Open Web",
    "description": "Censorship resistance, digital rights, and sovereign human communication.",
    "created_at": "2026-10-10T00:00:00Z"
  },
  {
    "slug": "g",
    "title": "Technology & Networks",
    "description": "P2P architectures, cryptography, overlays, and decentralized protocols.",
    "created_at": "2026-10-10T00:00:00Z"
  }
]
```

---

### 3. `GET /api/threads`
Lists active threads on a specified board, sorted with the most recently bumped threads first.

#### Query Parameters:
| Parameter | Type | Required | Default | Description |
|:---|:---:|:---:|:---:|:---|
| `board` | string | Optional | `"g"` | The board slug (e.g. `g`, `crypto`, `freedom`, `b`) |

#### Example Request:
```bash
curl -H "User-Agent: SovereignMesh-Client/1.0" \
     "http://127.0.0.1:8080/api/threads?board=g"
```

#### Example Response (HTTP 200):
```json
[
  {
    "id": 1,
    "thread_ref": "00a1b2c3d4e5f67890123456789abcdef0123456789abcdef0123456789abcdef",
    "board_slug": "g",
    "title": "Decentralized P2P Overlays vs Tor and I2P",
    "author_address": "mesh:455812a1763303dc542331ee39368d04100a5867",
    "post_count": 4,
    "created_at": "2026-10-10T01:15:30Z",
    "updated_at": "2026-10-10T01:45:12Z"
  }
]
```

---

### 4. `GET /api/posts`
Retrieves all posts (OP post followed by replies) within a specific thread in chronological order.

#### Query Parameters:
| Parameter | Type | Required | Description |
|:---|:---:|:---:|:---|
| `thread_id` | integer | **Required** | The numeric ID of the thread to retrieve |

#### Example Request:
```bash
curl -H "User-Agent: SovereignMesh-Client/1.0" \
     "http://127.0.0.1:8080/api/posts?thread_id=1"
```

#### Example Response (HTTP 200):
```json
[
  {
    "id": 1,
    "thread_id": 1,
    "thread_ref": "00a1b2c3d4...",
    "board_slug": "g",
    "title": "Decentralized P2P Overlays vs Tor and I2P",
    "author_address": "mesh:455812a1763303dc542331ee39368d04100a5867",
    "author_pubkey": "31016bb28f43b869a42eae84ae0a44d966989246355bfd135d425dffc06cbcbd",
    "content": "Can we build an overlay network that combines Reticulum addressing with Lokinet garlic routing?",
    "nonce": 1842,
    "pow_hash": "00a1b2c3d4e5f6...",
    "signature": "8a3ef4c1...",
    "timestamp": 1791594930,
    "is_op": true
  },
  {
    "id": 2,
    "thread_id": 1,
    "thread_ref": "00a1b2c3d4...",
    "board_slug": "g",
    "author_address": "mesh:1d344b04960914f8e0bd5c9632ba555d8d4862aa",
    "author_pubkey": "d07d2aa18cb95276ddfc9b506dd611cf5e2c48786807934651d8a8a5265703f1",
    "content": "Yes, using Argon2id anti-spam and content-addressed gossip.",
    "nonce": 420,
    "pow_hash": "00f3e2d1...",
    "signature": "1c7a9b02...",
    "timestamp": 1791596712,
    "is_op": false
  }
]
```
*Note: Post content served via HTTP includes invisible zero-width Unicode anti-scraping watermarks.*

---

### 5. `POST /api/posts`
Publishes a new thread (if `thread_id` is 0 or omitted) or posts a reply to an existing thread. The server automatically computes the Argon2id Proof-of-Work, signs the content with the node's private key, writes the post to local SQLite storage, and broadcasts it to connected P2P peers.

#### JSON Request Body:
```json
{
  "board_slug": "g",
  "title": "My New Thread Title (Optional for replies)",
  "content": "Message body to broadcast across the network.",
  "thread_id": 0,
  "thread_ref": ""
}
```

#### Example: Creating a New Thread
```bash
curl -X POST http://127.0.0.1:8080/api/posts \
     -H "Content-Type: application/json" \
     -H "User-Agent: SovereignMesh-Client/1.0" \
     -d '{
       "board_slug": "g",
       "title": "Decentralization Proof",
       "content": "Broadcasting genesis message across the P2P overlay."
     }'
```

#### Example Response (HTTP 200):
```json
{
  "duration": "1.234s",
  "nonce": 451,
  "post_id": 3,
  "pow_hash": "008f1c4e...",
  "status": "published",
  "thread_id": 2
}
```

#### Example: Replying to an Existing Thread
```bash
curl -X POST http://127.0.0.1:8080/api/posts \
     -H "Content-Type: application/json" \
     -H "User-Agent: SovereignMesh-Client/1.0" \
     -d '{
       "board_slug": "g",
       "thread_id": 2,
       "content": "Acknowledged and verified from node."
     }'
```

---

### 6. `GET /api/peers`
Returns a list of all known peer routing records in the local database.

#### Example Request:
```bash
curl -H "User-Agent: SovereignMesh-Client/1.0" \
     http://127.0.0.1:8080/api/peers
```

#### Example Response (HTTP 200):
```json
[
  {
    "address": "mesh:1d344b04960914f8e0bd5c9632ba555d8d4862aa",
    "endpoint": "127.0.0.1:9001",
    "last_seen": "2026-10-10T01:45:00Z",
    "connected": true
  }
]
```

---

### 7. `POST /api/peers/connect`
Commands the local node to dial an outbound P2P peer endpoint and perform the mutual cryptographic handshake.

#### JSON Request Body:
```json
{
  "endpoint": "192.0.2.1:9000"
}
```

#### Example Request:
```bash
curl -X POST http://127.0.0.1:8080/api/peers/connect \
     -H "Content-Type: application/json" \
     -H "User-Agent: SovereignMesh-Client/1.0" \
     -d '{"endpoint": "127.0.0.1:9001"}'
```

#### Example Response (HTTP 200):
```json
{
  "endpoint": "127.0.0.1:9001",
  "status": "connected"
}
```

---

## Rate-Limiting & Anti-Scrape Error Codes

| HTTP Status | Condition | Mitigation |
|:---:|:---|:---|
| `400 Bad Request` | Malformed JSON or empty message body. | Provide non-empty content and valid JSON formatting. |
| `405 Method Not Allowed` | Invalid HTTP verb used. | Use specified `GET` or `POST` verbs. |
| `429 Too Many Requests` | Burst rate limit exceeded (>30 req/min). | Back off requests or provide computational solution in `X-Mesh-PoW-Solution`. |
| `Connection Tarpitted` | Automated scraping tool signature detected or honeypot link hit. | Use standard browser header; avoid spidering `/api/canary/*` links. |
