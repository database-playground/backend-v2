package events

type EventType string

const (
	EventTypeLogin        EventType = "login"
	EventTypeImpersonated EventType = "impersonated"

	EventTypeSubmitAnswer EventType = "submit_answer"
)
