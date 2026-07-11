// Package cache provides a thread-safe, in-memory SHA-256 deduplication cache.
// Prompts are normalized by lowercasing and collapsing all whitespace
// (spaces, tabs, newlines) to ensure maximum cache-hit rates.
package cache

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"sync"
)

// Cache holds the thread-safe lookup map.
type Cache struct {
	store sync.Map
}

// New creates a new deduplication cache.
func New() *Cache {
	return &Cache{}
}

// Get checks the cache for an answer corresponding to the given prompt.
// Normalizes the prompt first before computing the lookup key.
func (c *Cache) Get(prompt string) (string, bool) {
	key := normalize(prompt)
	val, ok := c.store.Load(key)
	if !ok {
		return "", false
	}
	return val.(string), true
}

// Set stores the prompt's answer in the cache.
// Normalizes the prompt first before computing the lookup key.
func (c *Cache) Set(prompt string, answer string) {
	key := normalize(prompt)
	c.store.Store(key, answer)
}

// normalize normalizes a prompt for cache key matching.
// Converts to lowercase, trims whitespace, and collapses all whitespace runs
// (spaces, tabs, newlines) to single spaces.
func normalize(prompt string) string {
	s := strings.ToLower(strings.TrimSpace(prompt))
	s = strings.Join(strings.Fields(s), " ")
	hash := sha256.Sum256([]byte(s))
	return hex.EncodeToString(hash[:])
}
