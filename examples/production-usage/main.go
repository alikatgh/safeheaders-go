package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/alikatgh/safeheaders-go/jsmn-go"
)

const (
	// Server configuration
	ServerPort    = ":8080"
	ReadTimeout   = 10 * time.Second
	WriteTimeout  = 10 * time.Second
	IdleTimeout   = 60 * time.Second
	MaxHeaderSize = 1 << 20 // 1MB

	// Request limits
	MaxRequestSize = 10 * 1024 * 1024 // 10MB
	ParseTimeout   = 30 * time.Second
)

func main() {
	fmt.Println("SafeHeaders-Go Production Example")
	fmt.Println("==================================")
	fmt.Println()

	// Configure runtime
	runtime.GOMAXPROCS(runtime.NumCPU())

	// Create HTTP server
	srv := &http.Server{
		Addr:           ServerPort,
		Handler:        setupRoutes(),
		ReadTimeout:    ReadTimeout,
		WriteTimeout:   WriteTimeout,
		IdleTimeout:    IdleTimeout,
		MaxHeaderBytes: MaxHeaderSize,
	}

	// Start server in goroutine
	go func() {
		log.Printf("🚀 Server starting on %s", ServerPort)
		log.Printf("   Max CPUs: %d", runtime.NumCPU())
		log.Printf("   Max request size: %d MB", MaxRequestSize/(1024*1024))
		log.Println()
		log.Println("Endpoints:")
		log.Println("  POST /api/parse     - Parse JSON")
		log.Println("  GET  /health        - Health check")
		log.Println("  GET  /stats         - Server statistics")
		log.Println()

		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("\n🛑 Shutting down server...")

	// Graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited gracefully")
}

func setupRoutes() http.Handler {
	mux := http.NewServeMux()

	// API endpoints
	mux.HandleFunc("/api/parse", withMiddleware(handleParse))
	mux.HandleFunc("/health", withMiddleware(handleHealth))
	mux.HandleFunc("/stats", withMiddleware(handleStats))

	return mux
}

// Middleware chain
func withMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return logging(rateLimit(validateRequest(next)))
}

// Request validation middleware
func validateRequest(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Limit request body size
		r.Body = http.MaxBytesReader(w, r.Body, MaxRequestSize)
		next(w, r)
	}
}

// Rate limiting middleware (simple example)
var requestCount int64

func rateLimit(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestCount++
		// In production, use a proper rate limiter
		next(w, r)
	}
}

// Logging middleware
func logging(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		log.Printf("→ %s %s", r.Method, r.URL.Path)

		next(w, r)

		log.Printf("← %s %s (took %v)", r.Method, r.URL.Path, time.Since(start))
	}
}

// Handle JSON parsing requests
func handleParse(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read request body
	body := make([]byte, MaxRequestSize)
	n, err := r.Body.Read(body)
	if err != nil && err.Error() != "EOF" {
		http.Error(w, fmt.Sprintf("Failed to read body: %v", err), http.StatusBadRequest)
		return
	}
	body = body[:n]

	// Validate input
	if len(body) == 0 {
		http.Error(w, "Empty request body", http.StatusBadRequest)
		return
	}

	// Parse JSON with timeout
	ctx, cancel := context.WithTimeout(r.Context(), ParseTimeout)
	defer cancel()

	start := time.Now()
	tokens, err := jsmngo.ParseParallel(ctx, body)
	duration := time.Since(start)

	if err != nil {
		log.Printf("❌ Parse failed: %v", err)
		http.Error(w, fmt.Sprintf("Parse failed: %v", err), http.StatusBadRequest)
		return
	}

	// Build response
	response := map[string]interface{}{
		"status":     "success",
		"tokenCount": len(tokens),
		"inputSize":  len(body),
		"parseTime":  duration.String(),
		"timestamp":  time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)

	log.Printf("✅ Parsed %d tokens from %d bytes in %v", len(tokens), len(body), duration)
}

// Health check endpoint
func handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	health := map[string]interface{}{
		"status":    "healthy",
		"timestamp": time.Now().Format(time.RFC3339),
		"memory": map[string]interface{}{
			"allocMB":   m.Alloc / 1024 / 1024,
			"totalMB":   m.TotalAlloc / 1024 / 1024,
			"sysMB":     m.Sys / 1024 / 1024,
			"numGC":     m.NumGC,
			"goroutines": runtime.NumGoroutine(),
		},
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(health)
}

// Statistics endpoint
func handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := map[string]interface{}{
		"requestCount": requestCount,
		"cpus":         runtime.NumCPU(),
		"goVersion":    runtime.Version(),
		"timestamp":    time.Now().Format(time.RFC3339),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}
