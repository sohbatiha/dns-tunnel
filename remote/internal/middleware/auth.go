package middleware

import (
	"net/http"
	"sync"
)

// APIKeyAuth is a middleware that validates API keys
type APIKeyAuth struct {
	validKeys map[string]bool
	mu        sync.RWMutex
}

// NewAPIKeyAuth creates a new API key authentication middleware
func NewAPIKeyAuth(keys []string) *APIKeyAuth {
	auth := &APIKeyAuth{
		validKeys: make(map[string]bool),
	}
	for _, key := range keys {
		auth.validKeys[key] = true
	}
	return auth
}

// Middleware returns an HTTP middleware function
func (a *APIKeyAuth) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			apiKey = r.URL.Query().Get("api_key")
		}

		if !a.IsValidKey(apiKey) {
			http.Error(w, `{"error": "unauthorized", "message": "invalid or missing API key"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// IsValidKey checks if an API key is valid
func (a *APIKeyAuth) IsValidKey(key string) bool {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return a.validKeys[key]
}

// AddKey adds a new API key
func (a *APIKeyAuth) AddKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.validKeys[key] = true
}

// RemoveKey removes an API key
func (a *APIKeyAuth) RemoveKey(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.validKeys, key)
}
