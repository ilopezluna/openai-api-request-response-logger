package proxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"

	"openailogger/internal/config"
	"openailogger/storage"
)

// Gateway represents the capture gateway
type Gateway struct {
	config  *config.Config
	store   storage.Store
	workers chan *storage.Record
}

// New creates a new capture gateway
func New(cfg *config.Config, store storage.Store) *Gateway {
	g := &Gateway{
		config:  cfg,
		store:   store,
		workers: make(chan *storage.Record, cfg.Capture.WorkerPoolSize*2),
	}

	// Start worker pool for async storage
	for i := 0; i < cfg.Capture.WorkerPoolSize; i++ {
		go g.storageWorker()
	}

	return g
}

// ServeHTTP implements the main proxy handler
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Find matching route
	mount := g.extractMount(r.URL.Path)
	providerName, route, found := g.config.GetRouteByMount(mount)

	if !found {
		http.NotFound(w, r)
		return
	}

	// Parse upstream URL
	upstream, err := url.Parse(route.Upstream)
	if err != nil {
		http.Error(w, "Invalid upstream URL", http.StatusInternalServerError)
		return
	}

	// Create record for capture
	record := &storage.Record{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Provider:  providerName,
		Method:    r.Method,
		URL:       r.URL.String(),
		Upstream:  route.Upstream,
	}

	// Capture request body
	if err := g.captureRequestBody(r, record); err != nil {
		log.Printf("Failed to capture request body: %v", err)
		http.Error(w, "Failed to process request", http.StatusInternalServerError)
		return
	}

	// Create reverse proxy
	proxy := &httputil.ReverseProxy{
		Director: func(req *http.Request) {
			req.URL.Scheme = upstream.Scheme
			req.URL.Host = upstream.Host
			req.URL.Path = upstream.Path + strings.TrimPrefix(req.URL.Path, route.Mount)
			if req.URL.Path == "" {
				req.URL.Path = "/"
			}
		},
		ModifyResponse: func(resp *http.Response) error {
			record.Status = resp.StatusCode
			return g.captureResponseBody(resp, record)
		},
	}

	start := time.Now()
	proxy.ServeHTTP(w, r)
	record.DurationMS = time.Since(start).Milliseconds()

	// Extract model hint from request body
	g.extractModelHint(record)

	// Send to storage worker
	select {
	case g.workers <- record:
	default:
		log.Printf("Storage worker queue full, dropping record %s", record.ID)
	}
}

// captureRequestBody captures and buffers the request body
func (g *Gateway) captureRequestBody(r *http.Request, record *storage.Record) error {
	if r.Body == nil {
		return nil
	}

	// Read body with size limit
	maxBytes := g.config.MaxBodyBytes()
	body, err := io.ReadAll(io.LimitReader(r.Body, maxBytes))
	if err != nil {
		return fmt.Errorf("failed to read request body: %w", err)
	}

	record.RequestBody = string(body)
	record.SizeReqBytes = int64(len(body))

	// Replace body with a new reader for the proxy
	r.Body = io.NopCloser(bytes.NewReader(body))

	return nil
}

// captureResponseBody captures the response body while allowing streaming
func (g *Gateway) captureResponseBody(resp *http.Response, record *storage.Record) error {
	if resp.Body == nil {
		return nil
	}

	// Check if this is a streaming response
	contentType := resp.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/x-ndjson") ||
		resp.Header.Get("Transfer-Encoding") == "chunked"

	record.Stream = isStream

	var buf bytes.Buffer
	var chunks []string

	if isStream {
		// For streaming responses, capture chunks
		resp.Body = &streamCapture{
			reader:  resp.Body,
			buffer:  &buf,
			chunks:  &chunks,
			maxSize: g.config.MaxBodyBytes(),
		}
	} else {
		// For non-streaming responses, use a simple tee reader
		resp.Body = io.NopCloser(io.TeeReader(resp.Body, &buf))
	}

	// Set up a callback to capture the final data
	originalBody := resp.Body
	resp.Body = &bodyCapture{
		reader: originalBody,
		onClose: func() {
			record.ResponseBody = buf.String()
			record.SizeResBytes = int64(buf.Len())
			if len(chunks) > 0 {
				record.ResponseChunks = chunks
			}
		},
	}

	return nil
}

// extractMount extracts the mount path from a URL path
func (g *Gateway) extractMount(path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "/"), "/")
	if len(parts) > 0 {
		return "/" + parts[0]
	}
	return "/"
}

// extractModelHint attempts to extract model information from request body
func (g *Gateway) extractModelHint(record *storage.Record) {
	if record.RequestBody == "" {
		return
	}

	var data map[string]interface{}
	if err := json.Unmarshal([]byte(record.RequestBody), &data); err != nil {
		return
	}

	if model, ok := data["model"].(string); ok {
		record.ModelHint = model
	}
}

// storageWorker processes records for storage
func (g *Gateway) storageWorker() {
	for record := range g.workers {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		if err := g.store.Save(ctx, record); err != nil {
			log.Printf("Failed to save record %s: %v", record.ID, err)
		}
		cancel()
	}
}

// Close shuts down the gateway
func (g *Gateway) Close() error {
	close(g.workers)
	return g.store.Close()
}

// streamCapture captures streaming response data
type streamCapture struct {
	reader  io.ReadCloser
	buffer  *bytes.Buffer
	chunks  *[]string
	maxSize int64
}

func (sc *streamCapture) Read(p []byte) (n int, err error) {
	n, err = sc.reader.Read(p)
	if n > 0 {
		// Capture chunk if we haven't exceeded max size
		if int64(sc.buffer.Len()) < sc.maxSize {
			chunk := string(p[:n])
			*sc.chunks = append(*sc.chunks, chunk)
			sc.buffer.Write(p[:n])
		}
	}
	return n, err
}

func (sc *streamCapture) Close() error {
	return sc.reader.Close()
}

// bodyCapture wraps a reader to execute a callback on close
type bodyCapture struct {
	reader  io.ReadCloser
	onClose func()
	closed  bool
}

func (bc *bodyCapture) Read(p []byte) (n int, err error) {
	return bc.reader.Read(p)
}

func (bc *bodyCapture) Close() error {
	if !bc.closed {
		bc.closed = true
		bc.onClose()
	}
	return bc.reader.Close()
}
