package moderation

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

type BannedEntity struct {
	Identifier string    `json:"identifier"`
	Reason     string    `json:"reason"`
	BannedAt   time.Time `json:"banned_at"`
}

type DefederationManager struct {
	mu           sync.RWMutex
	bannedAddrs  map[string]BannedEntity
	bannedPubs   map[string]BannedEntity
	bannedHosts  map[string]BannedEntity
}

func NewDefederationManager() *DefederationManager {
	return &DefederationManager{
		bannedAddrs: make(map[string]BannedEntity),
		bannedPubs:  make(map[string]BannedEntity),
		bannedHosts: make(map[string]BannedEntity),
	}
}

// BanPeer permanently records a malicious actor in the local defederation blacklist.
func (dm *DefederationManager) BanPeer(address, pubKey, endpoint, reason string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	now := time.Now()
	entry := BannedEntity{
		Identifier: address,
		Reason:     reason,
		BannedAt:   now,
	}

	if address != "" {
		dm.bannedAddrs[address] = entry
	}
	if pubKey != "" {
		dm.bannedPubs[pubKey] = entry
	}
	if endpoint != "" {
		dm.bannedHosts[endpoint] = entry
	}
}

// IsBanned checks if an address, public key, or endpoint is on the defederation list.
func (dm *DefederationManager) IsBanned(address, pubKey, endpoint string) bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if address != "" && dm.bannedAddrs[address].Identifier != "" {
		return true
	}
	if pubKey != "" && dm.bannedPubs[pubKey].Identifier != "" {
		return true
	}
	if endpoint != "" && dm.bannedHosts[endpoint].Identifier != "" {
		return true
	}
	return false
}

// ValidatePost validates an inbound post against both cryptographic defederation lists and content filters.
func (dm *DefederationManager) ValidatePost(content, authorAddr, authorPubKey string) (bool, string) {
	if dm.IsBanned(authorAddr, authorPubKey, "") {
		return false, "Author identity is on defederation blacklist"
	}

	allowed, violation := ScanContent(content)
	if !allowed {
		// Automatically ban author upon first hate content violation
		dm.BanPeer(authorAddr, authorPubKey, "", violation)
		return false, violation
	}

	return true, ""
}

// ImportBlocklist parses and activates a community-curated defederation feed.
func (dm *DefederationManager) ImportBlocklist(jsonData []byte) (int, error) {
	var list struct {
		Version   string   `json:"version"`
		Addresses []string `json:"banned_addresses"`
		PubKeys   []string `json:"banned_pubkeys"`
		Endpoints []string `json:"banned_endpoints"`
		Reason    string   `json:"default_reason"`
	}

	if err := json.Unmarshal(jsonData, &list); err != nil {
		return 0, fmt.Errorf("invalid blocklist json: %w", err)
	}

	dm.mu.Lock()
	defer dm.mu.Unlock()

	count := 0
	now := time.Now()
	reason := list.Reason
	if reason == "" {
		reason = "Subscribed community defederation list"
	}

	for _, addr := range list.Addresses {
		dm.bannedAddrs[addr] = BannedEntity{Identifier: addr, Reason: reason, BannedAt: now}
		count++
	}
	for _, pub := range list.PubKeys {
		dm.bannedPubs[pub] = BannedEntity{Identifier: pub, Reason: reason, BannedAt: now}
		count++
	}
	for _, ep := range list.Endpoints {
		dm.bannedHosts[ep] = BannedEntity{Identifier: ep, Reason: reason, BannedAt: now}
		count++
	}

	return count, nil
}
