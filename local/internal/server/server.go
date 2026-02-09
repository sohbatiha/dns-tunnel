package server

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/miekg/dns"

	"github.com/mahdi/dns-proxy-local/internal/cache"
	"github.com/mahdi/dns-proxy-local/internal/client"
	"github.com/mahdi/dns-proxy-local/internal/config"
)

// Server represents the local DNS server
type Server struct {
	cfg       *config.Config
	udpServer *dns.Server
	tcpServer *dns.Server
	apiClient *client.Client
	cache     *cache.Cache
	logger    *log.Logger
}

// New creates a new DNS server
func New(cfg *config.Config, apiClient *client.Client) *Server {
	logger := log.New(os.Stdout, "[DNS-LOCAL] ", log.LstdFlags|log.Lshortfile)

	var dnsCache *cache.Cache
	if cfg.Cache.Enabled {
		dnsCache = cache.New(
			cfg.Cache.MaxItems,
			cfg.Cache.DefaultTTL,
			cfg.Cache.MinTTL,
			cfg.Cache.MaxTTL,
		)
	}

	return &Server{
		cfg:       cfg,
		apiClient: apiClient,
		cache:     dnsCache,
		logger:    logger,
	}
}

// Run starts the DNS server and blocks until shutdown
func (s *Server) Run() error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Server.ListenAddr, s.cfg.Server.Port)

	// Create DNS handler
	handler := dns.HandlerFunc(s.handleRequest)

	// Setup graceful shutdown
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	errChan := make(chan error, 2)

	// Start UDP server
	if s.cfg.Server.Protocol == "udp" || s.cfg.Server.Protocol == "both" {
		s.udpServer = &dns.Server{
			Addr:    addr,
			Net:     "udp",
			Handler: handler,
		}
		go func() {
			s.logger.Printf("Starting UDP DNS server on %s", addr)
			if err := s.udpServer.ListenAndServe(); err != nil {
				errChan <- fmt.Errorf("UDP server error: %w", err)
			}
		}()
	}

	// Start TCP server
	if s.cfg.Server.Protocol == "tcp" || s.cfg.Server.Protocol == "both" {
		s.tcpServer = &dns.Server{
			Addr:    addr,
			Net:     "tcp",
			Handler: handler,
		}
		go func() {
			s.logger.Printf("Starting TCP DNS server on %s", addr)
			if err := s.tcpServer.ListenAndServe(); err != nil {
				errChan <- fmt.Errorf("TCP server error: %w", err)
			}
		}()
	}

	// Wait for shutdown or error
	select {
	case <-stop:
		s.logger.Println("Shutting down DNS server...")
	case err := <-errChan:
		return err
	}

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if s.udpServer != nil {
		s.udpServer.ShutdownContext(ctx)
	}
	if s.tcpServer != nil {
		s.tcpServer.ShutdownContext(ctx)
	}

	return nil
}

func (s *Server) handleRequest(w dns.ResponseWriter, r *dns.Msg) {
	if len(r.Question) == 0 {
		return
	}

	q := r.Question[0]
	s.logger.Printf("Query: %s %s", q.Name, dns.TypeToString[q.Qtype])

	// Check cache
	if s.cache != nil {
		cacheKey := cache.Key(q)
		if cached, ok := s.cache.Get(cacheKey); ok {
			cached.Id = r.Id
			w.WriteMsg(cached)
			s.logger.Printf("Cache hit: %s", q.Name)
			return
		}
	}

	// Resolve via API
	resp, err := s.resolveViaAPI(r)
	if err != nil {
		s.logger.Printf("Resolution failed: %v", err)
		s.writeError(w, r, dns.RcodeServerFailure)
		return
	}

	// Cache response
	if s.cache != nil && len(resp.Answer) > 0 {
		cacheKey := cache.Key(q)
		s.cache.Set(cacheKey, resp)
	}

	w.WriteMsg(resp)
}

func (s *Server) resolveViaAPI(r *dns.Msg) (*dns.Msg, error) {
	q := r.Question[0]

	// Map DNS type
	recordType := dns.TypeToString[q.Qtype]

	// Call API
	ctx, cancel := context.WithTimeout(context.Background(), s.cfg.API.Timeout)
	defer cancel()

	result, err := s.apiClient.Resolve(ctx, strings.TrimSuffix(q.Name, "."), recordType)
	if err != nil {
		return nil, err
	}

	// Build DNS response
	resp := new(dns.Msg)
	resp.SetReply(r)
	resp.Authoritative = false
	resp.RecursionAvailable = true

	if result.Error != "" {
		resp.Rcode = dns.RcodeNameError
		return resp, nil
	}

	// Convert records to DNS RRs
	for _, rec := range result.Records {
		rr, err := s.createRR(rec, q.Name)
		if err != nil {
			s.logger.Printf("Failed to create RR: %v", err)
			continue
		}
		resp.Answer = append(resp.Answer, rr)
	}

	return resp, nil
}

func (s *Server) createRR(rec client.DNSRecord, name string) (dns.RR, error) {
	ttl := rec.TTL
	if ttl == 0 {
		ttl = 300
	}

	switch rec.Type {
	case "A":
		ip := net.ParseIP(rec.Value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IP: %s", rec.Value)
		}
		return &dns.A{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			A: ip.To4(),
		}, nil

	case "AAAA":
		ip := net.ParseIP(rec.Value)
		if ip == nil {
			return nil, fmt.Errorf("invalid IPv6: %s", rec.Value)
		}
		return &dns.AAAA{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeAAAA,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			AAAA: ip.To16(),
		}, nil

	case "CNAME":
		return &dns.CNAME{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeCNAME,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Target: dns.Fqdn(rec.Value),
		}, nil

	case "TXT":
		return &dns.TXT{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeTXT,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Txt: []string{rec.Value},
		}, nil

	case "MX":
		return &dns.MX{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeMX,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Preference: 10,
			Mx:         dns.Fqdn(rec.Value),
		}, nil

	case "NS":
		return &dns.NS{
			Hdr: dns.RR_Header{
				Name:   name,
				Rrtype: dns.TypeNS,
				Class:  dns.ClassINET,
				Ttl:    ttl,
			},
			Ns: dns.Fqdn(rec.Value),
		}, nil

	default:
		return nil, fmt.Errorf("unsupported record type: %s", rec.Type)
	}
}

func (s *Server) writeError(w dns.ResponseWriter, r *dns.Msg, rcode int) {
	resp := new(dns.Msg)
	resp.SetRcode(r, rcode)
	w.WriteMsg(resp)
}

// Stats returns server statistics
func (s *Server) Stats() map[string]interface{} {
	stats := map[string]interface{}{
		"api": s.apiClient.Stats(),
	}
	if s.cache != nil {
		stats["cache_size"] = s.cache.Len()
	}
	return stats
}
