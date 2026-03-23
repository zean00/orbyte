package notification

import (
	"fmt"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type Item struct {
	ID             string         `json:"id"`
	UserID         string         `json:"user_id"`
	Category       string         `json:"category"`
	Channel        string         `json:"channel,omitempty"`
	Status         string         `json:"status"`
	Title          string         `json:"title"`
	Body           string         `json:"body,omitempty"`
	TargetType     string         `json:"target_type,omitempty"`
	TargetID       string         `json:"target_id,omitempty"`
	DeepLinkPath   string         `json:"deep_link_path,omitempty"`
	ActionLinkPath string         `json:"action_link_path,omitempty"`
	Metadata       map[string]any `json:"metadata,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	ReadAt         time.Time      `json:"read_at,omitempty"`
	DismissedAt    time.Time      `json:"dismissed_at,omitempty"`
}

type Filter struct {
	UserID     string
	Category   string
	Status     string
	TargetType string
	TargetID   string
}

type Summary struct {
	Total     int `json:"total"`
	Unread    int `json:"unread"`
	Dismissed int `json:"dismissed"`
}

type Repository interface {
	Save(item Item) error
	Find(id string) (Item, bool)
	List(filter Filter) []Item
}

type Service struct {
	repo Repository
}

func NewService() *Service {
	return NewServiceWithRepository(NewMemoryRepository())
}

func NewServiceWithRepository(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Save(item Item) (Item, error) {
	if strings.TrimSpace(item.UserID) == "" {
		return Item{}, shared.Validation("notification user_id is required")
	}
	if strings.TrimSpace(item.Title) == "" {
		return Item{}, shared.Validation("notification title is required")
	}
	now := time.Now().UTC()
	item.ID = strings.TrimSpace(item.ID)
	if item.ID == "" {
		item.ID = fmt.Sprintf("notif:%d", now.UnixNano())
	}
	if item.Status == "" {
		item.Status = "unread"
	}
	if item.CreatedAt.IsZero() {
		item.CreatedAt = now
	}
	item.UserID = strings.TrimSpace(item.UserID)
	item.Category = strings.TrimSpace(item.Category)
	item.Channel = strings.TrimSpace(item.Channel)
	item.Status = normalizeStatus(item.Status)
	item.Title = strings.TrimSpace(item.Title)
	item.Body = strings.TrimSpace(item.Body)
	item.TargetType = strings.TrimSpace(item.TargetType)
	item.TargetID = strings.TrimSpace(item.TargetID)
	item.DeepLinkPath = strings.TrimSpace(item.DeepLinkPath)
	item.ActionLinkPath = strings.TrimSpace(item.ActionLinkPath)
	item.Metadata = cloneMap(item.Metadata)
	if err := s.repo.Save(item); err != nil {
		return Item{}, err
	}
	return cloneItem(item), nil
}

func (s *Service) Find(id string) (Item, bool) {
	return s.repo.Find(strings.TrimSpace(id))
}

func (s *Service) List(filter Filter) []Item {
	return s.repo.List(filter)
}

func (s *Service) Summary(userID string) Summary {
	items := s.List(Filter{UserID: strings.TrimSpace(userID)})
	out := Summary{Total: len(items)}
	for _, item := range items {
		switch item.Status {
		case "dismissed":
			out.Dismissed++
		case "read":
		default:
			out.Unread++
		}
	}
	return out
}

func (s *Service) MarkRead(id, userID string, now time.Time) (Item, error) {
	return s.updateStatus(id, userID, "read", now)
}

func (s *Service) Dismiss(id, userID string, now time.Time) (Item, error) {
	return s.updateStatus(id, userID, "dismissed", now)
}

func (s *Service) updateStatus(id, userID, status string, now time.Time) (Item, error) {
	id = strings.TrimSpace(id)
	userID = strings.TrimSpace(userID)
	item, ok := s.repo.Find(id)
	if !ok {
		return Item{}, shared.NotFound("notification not found")
	}
	if userID != "" && item.UserID != userID {
		return Item{}, shared.Forbidden("notification does not belong to the current user")
	}
	item.Status = normalizeStatus(status)
	switch item.Status {
	case "read":
		item.ReadAt = now.UTC()
		item.DismissedAt = time.Time{}
	case "dismissed":
		item.DismissedAt = now.UTC()
		if item.ReadAt.IsZero() {
			item.ReadAt = now.UTC()
		}
	}
	if err := s.repo.Save(item); err != nil {
		return Item{}, err
	}
	return cloneItem(item), nil
}

func normalizeStatus(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "read":
		return "read"
	case "dismissed":
		return "dismissed"
	default:
		return "unread"
	}
}

func cloneItem(item Item) Item {
	item.Metadata = cloneMap(item.Metadata)
	return item
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	out := make(map[string]any, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}
