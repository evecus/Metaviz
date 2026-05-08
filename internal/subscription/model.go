package subscription

import (
	"time"
)

// Subscription represents a proxy subscription URL.
type Subscription struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	UpdatedAt time.Time `json:"updatedAt"`
	NodeCount int       `json:"nodeCount"`
	Error     string    `json:"error,omitempty"`
}
