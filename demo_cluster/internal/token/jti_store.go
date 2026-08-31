package token

import (
	"sync"
	"time"
)

// JTIStore 进程内一次性票存储（Center 用这份即可多 Gate 共享）
type JTIStore struct {
	mu   sync.Mutex
	used map[string]int64 // jti -> expireAt unix ms
}

func NewJTIStore() *JTIStore {
	s := &JTIStore{used: make(map[string]int64)}
	go s.cleanupLoop()
	return s
}

// Consume 首次返回 true；已使用或空 jti 返回 false
func (s *JTIStore) Consume(jti string, ttlMs int64) bool {
	if jti == "" {
		return false
	}
	now := time.Now().UnixMilli()
	expireAt := now + ttlMs

	s.mu.Lock()
	defer s.mu.Unlock()

	if exp, ok := s.used[jti]; ok && exp > now {
		return false
	}
	s.used[jti] = expireAt
	return true
}

func (s *JTIStore) cleanupLoop() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now().UnixMilli()
		s.mu.Lock()
		for k, exp := range s.used {
			if exp <= now {
				delete(s.used, k)
			}
		}
		s.mu.Unlock()
	}
}
