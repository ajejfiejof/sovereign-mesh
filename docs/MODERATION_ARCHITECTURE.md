# Zero-Tolerance Moderation & Defederation Architecture 🛡️

## 1. The Core Philosophy
Sovereign Mesh was engineered to provide resilient, self-sovereign, and decentralized communication without corporate gatekeepers or mass surveillance. However, decentralized architecture must not become a refuge for violent extremism, fascism, or neo-Nazi organizing.

In centralized platforms, moderation is an administrative act of deleting database rows. In a decentralized peer-to-peer overlay, **moderation is an act of cryptographic quarantine and network defederation**.

---

## 2. The Four-Tier Enforcement Architecture

```
                            [ INBOUND GOSSIP PACKET ]
                                        │
                                        ▼
                   ┌───────────────────────────────────────────┐
                   │  1. INGESTION-LAYER SYMBOL & TEXT FILTER  │
                   │  Scans for 1488, swastikas, slurs, terror │
                   └────────────────────┬──────────────────────┘
                              Violated? │ Clean
                       ┌────────────────┴────────────────┐
                       ▼                                 ▼
         ┌───────────────────────────┐     ┌───────────────────────────┐
         │ 2. TAINT & SEVER DEFEDERATE│     │ 3. SUBSCRIBED BLOCKLISTS  │
         │ • Drop packet from memory │     │ Verify author against     │
         │ • Permanent address ban   │     │ community anti-hate feeds │
         │ • Sever TCP connection    │     └─────────────┬─────────────┘
         └───────────────────────────┘            Match? │ Clean
                                           ┌─────────────┴─────────────┐
                                           ▼                           ▼
                             ┌───────────────────────────┐ ┌───────────────────┐
                             │ REJECT & DROP FROM DB     │ │ WRITE TO SQLITE & │
                             │ Author added to blacklist │ │ GOSSIP TO PEERS   │
                             └───────────────────────────┘ └───────────────────┘
```

### Tier 1: Real-Time Symbol & Lexicon Scanning (`pkg/moderation/filter.go`)
Every post—whether submitted locally or received over P2P gossip—undergoes real-time inspection before Proof-of-Work verification or disk persistence:
- **Hate Slogans & Codes:** Immediate regex rejection of `1488`, `fourteen words`, `blood and soil`, `sieg heil`, and related slogans.
- **Paramilitary & Terror Groups:** Identifies neo-Nazi terrorist organizations (Atomwaffen, Combat 18, Blood & Honour, Nordic Resistance Movement).
- **Unicode Swastikas & Runes:** Inspects raw runes for Unicode swastikas (`\u534D`, `\u5350`) and Nazi runic variants (`\u16CB`).
- **Leetspeak Normalization:** Normalizes character substitutions (`b100d`, `@t0mw@ffen`) and strips invisible zero-width bypass tokens before evaluation.

### Tier 2: Taint-and-Sever Defederation (`pkg/moderation/defederate.go`)
Peer-to-peer propagation depends on nodes relaying traffic for each other. When a peer transmits a packet that violates the zero-tolerance policy:
1. **Memory Drop:** The packet is dropped in memory and never written to disk.
2. **Permanent Address Ban:** The author's cryptographic address (`mesh:<hex>`) and public key are marked as permanently hostile.
3. **Connection Sever:** The TCP socket to the transmitting peer is closed.
4. **Network Isolation:** Hostile nodes are cordoned into partitioned islands, unable to utilize the compliant mesh for relay.

### Tier 3: Subscribed Community Defederation Lists
Node operators can subscribe to community-maintained defederation lists (JSON feeds) published by trusted anti-hate research organizations:
```json
{
  "version": "1.0",
  "banned_addresses": ["mesh:badactor123..."],
  "banned_pubkeys": ["deadbeef..."],
  "banned_endpoints": ["198.51.100.1:9000"],
  "default_reason": "Identified neo-Nazi organizing node"
}
```
Any incoming connection or post signed by a blacklisted entity is dropped at the connection layer.

### Tier 4: Board Owner Cryptographic Tombstones
Boards created on Sovereign Mesh carry a genesis public key (`board_owner_pubkey`). Board creators can broadcast signed cryptographic `Tombstone` packets to retroactively purge violating content from all nodes subscribed to that board.
