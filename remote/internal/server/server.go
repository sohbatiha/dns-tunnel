package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/mahdi/dns-proxy-remote/internal/config"
	"github.com/mahdi/dns-proxy-remote/internal/crypto"
	"github.com/mahdi/dns-proxy-remote/internal/handler"
	"github.com/mahdi/dns-proxy-remote/internal/middleware"
	"github.com/mahdi/dns-proxy-remote/internal/resolver"
)

// Server represents the HTTPS DNS API server
type Server struct {
	cfg        *config.Config
	httpServer *http.Server
	resolver   *resolver.Resolver
	logger     *log.Logger
}

// New creates a new Server instance
func New(cfg *config.Config) (*Server, error) {
	logger := log.New(os.Stdout, "[DNS-API] ", log.LstdFlags|log.Lshortfile)

	// Create resolver
	res := resolver.New(resolver.Config{
		Upstreams:     cfg.Resolver.Upstreams,
		Timeout:       cfg.Resolver.Timeout,
		MaxRetries:    cfg.Resolver.MaxRetries,
		CacheEnabled:  cfg.Resolver.CacheEnabled,
		CacheTTL:      cfg.Resolver.CacheTTL,
		CacheMaxItems: cfg.Resolver.CacheMaxItems,
	})

	// Create cipher if encryption is enabled
	var cipher *crypto.Cipher
	if cfg.Security.EncryptionEnabled {
		var err error
		cipher, err = crypto.NewCipher(cfg.Security.EncryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to create cipher: %w", err)
		}
	}

	// Create handler
	h := handler.NewHandler(res, cipher)

	// Create router
	mux := http.NewServeMux()

	// Public endpoints (no auth required)
	mux.HandleFunc("/health", h.Health)

	// Protected endpoints
	protectedMux := http.NewServeMux()
	protectedMux.HandleFunc("/api/v1/resolve", h.Resolve)
	protectedMux.HandleFunc("/api/v1/data", h.Resolve) // Obfuscated endpoint

	// Apply middleware chain
	var protectedHandler http.Handler = protectedMux

	// Rate limiting
	if cfg.Security.RateLimitEnabled {
		rateLimiter := middleware.NewRateLimiter(cfg.Security.RateLimitPerSec, cfg.Security.RateLimitBurst)
		protectedHandler = rateLimiter.Middleware(protectedHandler)
	}

	// API key authentication
	auth := middleware.NewAPIKeyAuth(cfg.Security.APIKeys)
	protectedHandler = auth.Middleware(protectedHandler)

	// Add logging middleware
	protectedHandler = loggingMiddleware(logger, protectedHandler)

	// Mount protected routes
	mux.Handle("/api/", protectedHandler)

	// Create HTTP server
	addr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	httpServer := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
			CipherSuites: []uint16{
				tls.TLS_ECDHE_ECDSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_RSA_WITH_AES_256_GCM_SHA384,
				tls.TLS_ECDHE_ECDSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_RSA_WITH_CHACHA20_POLY1305,
				tls.TLS_ECDHE_ECDSA_WITH_AES_128_GCM_SHA256,
				tls.TLS_ECDHE_RSA_WITH_AES_128_GCM_SHA256,
			},
		},
	}

	return &Server{
		cfg:        cfg,
		httpServer: httpServer,
		resolver:   res,
		logger:     logger,
	}, nil
}

// Run starts the server and blocks until shutdown
func (s *Server) Run() error {
	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Start server
	go func() {
		s.logger.Printf("Starting HTTPS server on %s", s.httpServer.Addr)
		var err error
		if s.cfg.Server.TLSCertFile != "" && s.cfg.Server.TLSKeyFile != "" {
			err = s.httpServer.ListenAndServeTLS(s.cfg.Server.TLSCertFile, s.cfg.Server.TLSKeyFile)
		} else {
			s.logger.Println("WARNING: Running without TLS (development mode only)")
			err = s.httpServer.ListenAndServe()
		}
		if err != nil && err != http.ErrServerClosed {
			s.logger.Fatalf("Server error: %v", err)
		}
	}()

	// Wait for shutdown signal
	<-stop
	s.logger.Println("Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	return s.httpServer.Shutdown(ctx)
}

func loggingMiddleware(logger *log.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap response writer to capture status code
		wrapped := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}

		next.ServeHTTP(wrapped, r)

		logger.Printf("%s %s %d %s",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			time.Since(start),
		)
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
