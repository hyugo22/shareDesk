// Package ratelimit fournit une limitation de débit en mémoire (par clé,
// typiquement l'IP source) pour protéger les endpoints sensibles (login,
// enrôlement d'agent) contre les abus par force brute.
package ratelimit

import (
	"sync"
	"time"
)

type bucket struct {
	count     int
	windowEnd time.Time
}

type Limiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	limit   int
	window  time.Duration
}

func New(limit int, window time.Duration) *Limiter {
	l := &Limiter{buckets: make(map[string]*bucket), limit: limit, window: window}
	go l.gc()
	return l
}

// Allow retourne false si la clé a dépassé le nombre de requêtes autorisées sur la fenêtre courante.
func (l *Limiter) Allow(key string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	b, ok := l.buckets[key]
	if !ok || now.After(b.windowEnd) {
		l.buckets[key] = &bucket{count: 1, windowEnd: now.Add(l.window)}
		return true
	}
	if b.count >= l.limit {
		return false
	}
	b.count++
	return true
}

func (l *Limiter) gc() {
	ticker := time.NewTicker(l.window)
	defer ticker.Stop()
	for range ticker.C {
		l.mu.Lock()
		now := time.Now()
		for k, b := range l.buckets {
			if now.After(b.windowEnd) {
				delete(l.buckets, k)
			}
		}
		l.mu.Unlock()
	}
}
