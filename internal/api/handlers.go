package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"openailogger/storage"
)

// Handler provides REST API endpoints for the capture data
type Handler struct {
	store storage.Store
}

// New creates a new API handler
func New(store storage.Store) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes registers all API routes with the given mux
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/requests", h.handleRequests)
	mux.HandleFunc("/api/requests/", h.handleRequestByID)
	mux.HandleFunc("/api/export.ndjson", h.handleExport)
}

// handleRequests handles GET /api/requests with filtering and pagination
func (h *Handler) handleRequests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query, err := h.parseQuery(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	records, total, err := h.store.List(r.Context(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list records: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"records": records,
		"total":   total,
		"offset":  query.Offset,
		"limit":   query.Limit,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleRequestByID handles individual request operations
func (h *Handler) handleRequestByID(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	path := strings.TrimPrefix(r.URL.Path, "/api/requests/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		http.Error(w, "Missing request ID", http.StatusBadRequest)
		return
	}

	id := parts[0]

	switch r.Method {
	case http.MethodGet:
		if len(parts) > 1 && parts[1] == "chunks" {
			h.handleRequestChunks(w, r, id)
		} else {
			h.handleGetRequest(w, r, id)
		}
	case http.MethodDelete:
		h.handleDeleteRequest(w, r, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleGetRequest handles GET /api/requests/{id}
func (h *Handler) handleGetRequest(w http.ResponseWriter, r *http.Request, id string) {
	record, err := h.store.Get(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Record not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to get record: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(record)
}

// handleRequestChunks handles GET /api/requests/{id}/chunks for stream playback
func (h *Handler) handleRequestChunks(w http.ResponseWriter, r *http.Request, id string) {
	record, err := h.store.Get(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Record not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to get record: %v", err), http.StatusInternalServerError)
		}
		return
	}

	if !record.Stream || len(record.ResponseChunks) == 0 {
		http.Error(w, "No chunks available for this record", http.StatusNotFound)
		return
	}

	// Stream chunks back to client
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	for i, chunk := range record.ResponseChunks {
		fmt.Fprintf(w, "data: %s\n\n", chunk)
		flusher.Flush()

		// Add small delay between chunks for realistic playback
		if i < len(record.ResponseChunks)-1 {
			time.Sleep(50 * time.Millisecond)
		}
	}
}

// handleDeleteRequest handles DELETE /api/requests/{id}
func (h *Handler) handleDeleteRequest(w http.ResponseWriter, r *http.Request, id string) {
	err := h.store.Delete(r.Context(), id)
	if err != nil {
		if strings.Contains(err.Error(), "not found") {
			http.Error(w, "Record not found", http.StatusNotFound)
		} else {
			http.Error(w, fmt.Sprintf("Failed to delete record: %v", err), http.StatusInternalServerError)
		}
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// handleExport handles GET /api/export.ndjson
func (h *Handler) handleExport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query, err := h.parseQuery(r)
	if err != nil {
		http.Error(w, fmt.Sprintf("Invalid query parameters: %v", err), http.StatusBadRequest)
		return
	}

	// Remove pagination for export
	query.Limit = 0
	query.Offset = 0

	reader, err := h.store.ExportNDJSON(r.Context(), query)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to export records: %v", err), http.StatusInternalServerError)
		return
	}
	defer reader.Close()

	w.Header().Set("Content-Type", "application/x-ndjson")
	w.Header().Set("Content-Disposition", "attachment; filename=capture-export.ndjson")

	io.Copy(w, reader)
}

// parseQuery parses query parameters into a storage.Query
func (h *Handler) parseQuery(r *http.Request) (storage.Query, error) {
	query := storage.Query{
		Limit: 50,    // Default limit
		Sort:  "-ts", // Default sort (newest first)
	}

	params := r.URL.Query()

	// Provider filter
	if provider := params.Get("provider"); provider != "" {
		query.Provider = &provider
	}

	// Model filter
	if model := params.Get("modelLike"); model != "" {
		query.ModelLike = &model
	}

	// URL filter
	if urlLike := params.Get("urlLike"); urlLike != "" {
		query.URLLike = &urlLike
	}

	// Status filter
	if statusStr := params.Get("status"); statusStr != "" {
		status, err := strconv.Atoi(statusStr)
		if err != nil {
			return query, fmt.Errorf("invalid status parameter: %v", err)
		}
		query.StatusEq = &status
	}

	// Text search
	if q := params.Get("q"); q != "" {
		query.TextSearch = &q
	}

	// Time range filters
	if fromStr := params.Get("from"); fromStr != "" {
		from, err := time.Parse(time.RFC3339, fromStr)
		if err != nil {
			return query, fmt.Errorf("invalid from parameter: %v", err)
		}
		query.From = &from
	}

	if toStr := params.Get("to"); toStr != "" {
		to, err := time.Parse(time.RFC3339, toStr)
		if err != nil {
			return query, fmt.Errorf("invalid to parameter: %v", err)
		}
		query.To = &to
	}

	// Pagination
	if offsetStr := params.Get("offset"); offsetStr != "" {
		offset, err := strconv.Atoi(offsetStr)
		if err != nil {
			return query, fmt.Errorf("invalid offset parameter: %v", err)
		}
		query.Offset = offset
	}

	if limitStr := params.Get("limit"); limitStr != "" {
		limit, err := strconv.Atoi(limitStr)
		if err != nil {
			return query, fmt.Errorf("invalid limit parameter: %v", err)
		}
		if limit > 0 {
			query.Limit = limit
		}
	}

	// Sort
	if sort := params.Get("sort"); sort != "" {
		if sort == "ts" || sort == "-ts" {
			query.Sort = sort
		} else {
			return query, fmt.Errorf("invalid sort parameter: must be 'ts' or '-ts'")
		}
	}

	return query, nil
}
