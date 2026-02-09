package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"time"

	"github.com/mahdi/dns-proxy-remote/internal/crypto"
	"github.com/mahdi/dns-proxy-remote/internal/resolver"
)

// ResolveRequest represents the incoming DNS resolution request
type ResolveRequest struct {
	Domain    string `json:"domain"`
	Type      string `json:"type"`
	Encrypted string `json:"encrypted,omitempty"` // Base64 encoded encrypted payload
}

// ResolveResponse represents the DNS resolution response
type ResolveResponse struct {
	Domain  string               `json:"domain"`
	Records []resolver.DNSRecord `json:"records"`
	Cached  bool                 `json:"cached"`
	Error   string               `json:"error,omitempty"`
}

// EncryptedRequest represents an encrypted request payload
type EncryptedRequest struct {
	Data string `json:"data"` // Base64 encoded encrypted JSON
}

// Handler handles DNS resolution HTTP requests
type Handler struct {
	resolver *resolver.Resolver
	cipher   *crypto.Cipher
}

// NewHandler creates a new DNS resolution handler
func NewHandler(resolver *resolver.Resolver, cipher *crypto.Cipher) *Handler {
	return &Handler{
		resolver: resolver,
		cipher:   cipher,
	}
}

// Resolve handles POST /api/v1/resolve
func (h *Handler) Resolve(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req ResolveRequest

	// Handle encrypted payload if cipher is configured
	if h.cipher != nil {
		var encReq EncryptedRequest
		if err := json.NewDecoder(r.Body).Decode(&encReq); err != nil {
			h.writeError(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if encReq.Data != "" {
			decrypted, err := h.cipher.Decrypt(encReq.Data)
			if err != nil {
				h.writeError(w, "decryption failed", http.StatusBadRequest)
				return
			}
			if err := json.Unmarshal(decrypted, &req); err != nil {
				h.writeError(w, "invalid decrypted payload", http.StatusBadRequest)
				return
			}
		} else {
			// Fallback to unencrypted (for backwards compatibility)
			if err := json.Unmarshal([]byte(r.Body.Read), &req); err != nil {
				h.writeError(w, "invalid request body", http.StatusBadRequest)
				return
			}
		}
	} else {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, "invalid request body", http.StatusBadRequest)
			return
		}
	}

	// Validate request
	if req.Domain == "" {
		h.writeError(w, "domain is required", http.StatusBadRequest)
		return
	}

	// Default to A record if not specified
	recordType := resolver.TypeA
	if req.Type != "" {
		recordType = resolver.RecordType(strings.ToUpper(req.Type))
	}

	// Resolve DNS
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	result, err := h.resolver.Resolve(ctx, req.Domain, recordType)
	if err != nil {
		h.writeJSON(w, ResolveResponse{
			Domain: req.Domain,
			Error:  err.Error(),
		}, http.StatusOK)
		return
	}

	h.writeJSON(w, ResolveResponse{
		Domain:  result.Domain,
		Records: result.Records,
		Cached:  result.Cached,
	}, http.StatusOK)
}

// Health handles GET /health
func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	h.writeJSON(w, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
		"stats":  h.resolver.Stats(),
	}, http.StatusOK)
}

func (h *Handler) writeError(w http.ResponseWriter, message string, status int) {
	h.writeJSON(w, map[string]string{"error": message}, status)
}

func (h *Handler) writeJSON(w http.ResponseWriter, data interface{}, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
