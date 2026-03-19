package secretstore

import "time"

type Secret struct {
	Ref       string    `json:"ref"`
	Name      string    `json:"name"`
	Value     string    `json:"-"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
