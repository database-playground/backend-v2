package events

type EventType string

const (
	EventTypeLogin        EventType = "login"
	EventTypeImpersonated EventType = "impersonated"
	EventTypeLogout       EventType = "logout"
	EventTypeLogoutAll    EventType = "logout_all"

	EventTypeSubmitAnswer EventType = "submit_answer"

	// Internal usage
	EventTypeGrantPoint EventType = "grant_point"
)
