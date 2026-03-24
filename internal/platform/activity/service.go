package activity

import (
	"sort"
	"strings"
	"time"

	"orbyte/internal/platform/shared"
)

type Message struct {
	ID         string         `json:"id"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	AuthorID   string         `json:"author_id"`
	Body       string         `json:"body"`
	Metadata   map[string]any `json:"metadata,omitempty"`
	CreatedAt  time.Time      `json:"created_at"`
}

type Follower struct {
	ID         string    `json:"id"`
	TargetType string    `json:"target_type"`
	TargetID   string    `json:"target_id"`
	ActorID    string    `json:"actor_id"`
	CreatedAt  time.Time `json:"created_at"`
}

type Activity struct {
	ID          string    `json:"id"`
	TargetType  string    `json:"target_type"`
	TargetID    string    `json:"target_id"`
	Summary     string    `json:"summary"`
	Status      string    `json:"status"`
	AssignedTo  string    `json:"assigned_to,omitempty"`
	CreatedBy   string    `json:"created_by"`
	CreatedAt   time.Time `json:"created_at"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

type TimelineEntry struct {
	Kind       string         `json:"kind"`
	ID         string         `json:"id"`
	TargetType string         `json:"target_type"`
	TargetID   string         `json:"target_id"`
	OccurredAt time.Time      `json:"occurred_at"`
	Payload    map[string]any `json:"payload"`
}

type Service struct {
	messages   []Message
	followers  []Follower
	activities []Activity
}

func NewService() *Service {
	return &Service{
		messages:   []Message{},
		followers:  []Follower{},
		activities: []Activity{},
	}
}

func (s *Service) AddMessage(targetType, targetID, authorID, body string, metadata map[string]any) (Message, error) {
	if strings.TrimSpace(targetType) == "" || strings.TrimSpace(targetID) == "" || strings.TrimSpace(body) == "" {
		return Message{}, shared.Validation("target_type, target_id, and body are required")
	}
	now := time.Now().UTC()
	item := Message{
		ID:         shared.NewID("msg"),
		TargetType: strings.TrimSpace(targetType),
		TargetID:   strings.TrimSpace(targetID),
		AuthorID:   fallbackActor(authorID),
		Body:       strings.TrimSpace(body),
		Metadata:   cloneMap(metadata),
		CreatedAt:  now,
	}
	s.messages = append(s.messages, item)
	return item, nil
}

func (s *Service) Follow(targetType, targetID, actorID string) (Follower, error) {
	if strings.TrimSpace(targetType) == "" || strings.TrimSpace(targetID) == "" || strings.TrimSpace(actorID) == "" {
		return Follower{}, shared.Validation("target_type, target_id, and actor_id are required")
	}
	now := time.Now().UTC()
	item := Follower{ID: shared.NewID("follow"), TargetType: targetType, TargetID: targetID, ActorID: actorID, CreatedAt: now}
	s.followers = append(s.followers, item)
	return item, nil
}

func (s *Service) CreateActivity(targetType, targetID, createdBy, assignedTo, summary string) (Activity, error) {
	if strings.TrimSpace(targetType) == "" || strings.TrimSpace(targetID) == "" || strings.TrimSpace(summary) == "" {
		return Activity{}, shared.Validation("target_type, target_id, and summary are required")
	}
	now := time.Now().UTC()
	item := Activity{
		ID:         shared.NewID("act"),
		TargetType: targetType,
		TargetID:   targetID,
		Summary:    strings.TrimSpace(summary),
		Status:     "open",
		AssignedTo: strings.TrimSpace(assignedTo),
		CreatedBy:  fallbackActor(createdBy),
		CreatedAt:  now,
	}
	s.activities = append(s.activities, item)
	return item, nil
}

func (s *Service) CompleteActivity(id string) (Activity, error) {
	for i := range s.activities {
		if s.activities[i].ID != strings.TrimSpace(id) {
			continue
		}
		s.activities[i].Status = "completed"
		s.activities[i].CompletedAt = time.Now().UTC()
		return s.activities[i], nil
	}
	return Activity{}, shared.NotFound("activity not found")
}

func (s *Service) Timeline(targetType, targetID string) []TimelineEntry {
	items := make([]TimelineEntry, 0)
	for _, item := range s.messages {
		if item.TargetType != targetType || item.TargetID != targetID {
			continue
		}
		items = append(items, TimelineEntry{
			Kind: "message", ID: item.ID, TargetType: item.TargetType, TargetID: item.TargetID, OccurredAt: item.CreatedAt,
			Payload: map[string]any{"author_id": item.AuthorID, "body": item.Body, "metadata": item.Metadata},
		})
	}
	for _, item := range s.activities {
		if item.TargetType != targetType || item.TargetID != targetID {
			continue
		}
		items = append(items, TimelineEntry{
			Kind: "activity", ID: item.ID, TargetType: item.TargetType, TargetID: item.TargetID, OccurredAt: item.CreatedAt,
			Payload: map[string]any{"summary": item.Summary, "status": item.Status, "assigned_to": item.AssignedTo, "completed_at": item.CompletedAt},
		})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].OccurredAt.Before(items[j].OccurredAt) })
	return items
}

func fallbackActor(actorID string) string {
	if strings.TrimSpace(actorID) == "" {
		return "system"
	}
	return strings.TrimSpace(actorID)
}

func cloneMap(input map[string]any) map[string]any {
	if len(input) == 0 {
		return map[string]any{}
	}
	output := make(map[string]any, len(input))
	for key, value := range input {
		output[key] = value
	}
	return output
}
