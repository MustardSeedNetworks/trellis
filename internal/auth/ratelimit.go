// SPDX-License-Identifier: BUSL-1.1

package auth

import (
	"sync"
	"time"
)

// MaxTrackedClients caps how many client addresses the limiter remembers. The
// address comes from the request, so without a cap an attacker choosing a fresh
// source per attempt would grow the map until the daemon died — which is a
// denial of service reached through the thing meant to prevent one. At the cap
// the oldest entries are dropped; a client whose entry is dropped gets a fresh
// budget, which is the safe direction to fail for a survey tool.
const MaxTrackedClients = 4096

// RateLimiter is a fixed-window per-client attempt counter.
type RateLimiter struct {
	limit  int
	window time.Duration

	mu      sync.Mutex
	clients map[string]*window
}

type window struct {
	count int
	start time.Time
}

// NewRateLimiter returns a limiter allowing limit attempts per window.
func NewRateLimiter(limit int, w time.Duration) *RateLimiter {
	return &RateLimiter{limit: limit, window: w, clients: make(map[string]*window)}
}

// Allow records an attempt by client and reports whether it is within budget.
func (l *RateLimiter) Allow(client string) bool {
	now := time.Now()
	l.mu.Lock()
	defer l.mu.Unlock()

	w, ok := l.clients[client]
	if !ok || now.Sub(w.start) >= l.window {
		l.evictIfFull(now)
		l.clients[client] = &window{count: 1, start: now}
		return true
	}
	if w.count >= l.limit {
		return false
	}
	w.count++
	return true
}

// Reset returns a client's budget, called after a successful authentication so
// a slow-typing operator is not locked out by their own corrections.
func (l *RateLimiter) Reset(client string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.clients, client)
}

// Tracked reports how many clients the limiter currently holds.
func (l *RateLimiter) Tracked() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return len(l.clients)
}

// evictIfFull drops expired entries when the map is at capacity, and if that
// frees nothing, drops an arbitrary entry so the map can never exceed the cap.
// Callers hold l.mu.
func (l *RateLimiter) evictIfFull(now time.Time) {
	if len(l.clients) < MaxTrackedClients {
		return
	}
	for addr, w := range l.clients {
		if now.Sub(w.start) >= l.window {
			delete(l.clients, addr)
		}
	}
	for addr := range l.clients {
		if len(l.clients) < MaxTrackedClients {
			break
		}
		delete(l.clients, addr)
	}
}
