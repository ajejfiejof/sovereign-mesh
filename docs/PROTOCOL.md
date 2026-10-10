# Sovereign Mesh Wire Protocol Specification (v1.0)

## 1. Overview
Sovereign Mesh is an application-level peer-to-peer overlay designed for decentralized, censorship-resistant messageboards. Nodes communicate directly over TCP without reliance on ICANN domain names, certificate authorities, or centralized coordinators.

---

## 2. Cryptographic Addressing & Identity
- **Signature Algorithm:** `Ed25519` (RFC 8032)
- **Node Address Derivation:**
  $$\text{Address} = \text{"mesh:"} + \text{Hex}(\text{SHA-256}(\text{Ed25519\_PublicKey})[0:20])$$
  Yields a persistent 45-character self-sovereign network identifier.
- **Mutual Authentication:** Nodes do not accept unauthenticated connections. Upon TCP connection establishment, both peers must sign an ephemeral UTC timestamp to prove private key possession.

---

## 3. Binary Packet Framing

All stream communications use framed binary packets:

```
+----------------+----------------+--------------------------+------------------------+
| Magic (2B)     | MsgType (1B)   | Payload Length (4B, BE)  | Payload (0 to 4MB)     |
| 0x53 0x4D      | 0x01 to 0x06   | uint32 Big-Endian        | Variable raw bytes     |
+----------------+----------------+--------------------------+------------------------+
```

### Magic Bytes
- `0x53 0x4D` (ASCII: `"SM"` for SovereignMesh). Packets failing to match are dropped immediately.

### Message Type Opcodes
| Opcode | Identifier | Description |
|:---:|:---|:---|
| `0x01` | `MsgTypeHandshake` | Initial cryptographic challenge payload |
| `0x02` | `MsgTypeHandshakeAck` | Challenge acknowledgment and confirmation |
| `0x03` | `MsgTypeGossipPost` | Mined, signed post payload for gossip broadcast |
| `0x04` | `MsgTypePing` | Heartbeat keepalive ping (30s interval) |
| `0x05` | `MsgTypePong` | Heartbeat keepalive response |
| `0x06` | `MsgTypePeerExchange` | PEX list of known active peer endpoints |

---

## 4. Anti-Spam Mining: Memory-Hard Argon2id
To prevent botnet floods without CAPTCHAs, every published post must satisfy an Argon2id Proof-of-Work puzzle:

- **Algorithm:** `Argon2id` (RFC 9106)
- **Time Cost:** `t = 1` iteration
- **Memory Cost:** `m = 32,768 KB` (32 MB RAM required per hash attempt)
- **Parallelism:** `p = 2` threads
- **Key Length:** `32` bytes
- **Domain Salt:** `"SovereignMeshAntiSpamV1"`

### Mining Header
$$\text{Header} = \text{Content} + \text{":"} + \text{Nonce}$$

### Verification Rule
A post is accepted if:
$$\text{Hex}(\text{Argon2id}(\text{Header})).\text{startswith}(\text{"0"}^{\text{Difficulty}})$$

---

## 5. Decentralized Thread Replication
To prevent database conflicts across nodes without a central server:
- **`ThreadRef`:** An OP post's thread identifier is derived from its unique cryptographic PoW hash.
- **Replies:** Replies reference the OP's `ThreadRef`.
- **Peer Ingestion:** When a node receives a post via gossip, it resolves the thread by `ThreadRef` or automatically creates the thread if it is the first post seen.
- **Ephemeral Pruning:** Boards maintain a maximum of 50 active threads. When the limit is exceeded, the least recently bumped threads are automatically purged.
