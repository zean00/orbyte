package application

type ActingContext struct {
	ActorID           string
	ActorKind         string
	EffectiveUserID   string
	OnBehalfOfUserID  string
	DelegationGrantID string
}

func (c ActingContext) EffectiveActorID() string {
	if c.EffectiveUserID != "" {
		return c.EffectiveUserID
	}
	return c.ActorID
}
