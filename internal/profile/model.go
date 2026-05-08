package profile

import "time"

// Profile represents an uploaded config file entry.
type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Filename  string    `json:"filename"`
	UpdatedAt time.Time `json:"updatedAt"`
}
