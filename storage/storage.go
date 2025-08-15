package storage

import (
	"context"
	"io"
	"time"
)

// Record represents a captured request/response pair
type Record struct {
	ID             string    `json:"id"`
	Timestamp      time.Time `json:"ts"`
	Provider       string    `json:"provider"`
	Method         string    `json:"method"`
	URL            string    `json:"url"`
	Upstream       string    `json:"upstream"`
	Status         int       `json:"status"`
	DurationMS     int64     `json:"duration_ms"`
	RequestBody    string    `json:"request_body"`
	ResponseBody   string    `json:"response_body"`
	Stream         bool      `json:"stream"`
	ResponseChunks []string  `json:"response_chunks,omitempty"`
	SizeReqBytes   int64     `json:"size_req_bytes"`
	SizeResBytes   int64     `json:"size_res_bytes"`
	ModelHint      string    `json:"model_hint,omitempty"`
	Error          *string   `json:"error,omitempty"`
}

// Query represents search/filter parameters for records
type Query struct {
	Provider   *string
	ModelLike  *string
	URLLike    *string
	StatusEq   *int
	From       *time.Time
	To         *time.Time
	TextSearch *string
	Offset     int
	Limit      int
	Sort       string // "ts" or "-ts"
}

// Store defines the interface for storage backends
type Store interface {
	Save(ctx context.Context, r *Record) error
	Get(ctx context.Context, id string) (*Record, error)
	List(ctx context.Context, q Query) ([]Record, int, error)
	Delete(ctx context.Context, id string) error
	ExportNDJSON(ctx context.Context, q Query) (io.ReadCloser, error)
	Close() error
}
