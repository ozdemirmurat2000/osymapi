package models

import "time"

type Publisher struct {
	ID         int       `json:"id"`
	Name       string    `json:"name"`
	WebsiteURL string    `json:"website_url,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}
