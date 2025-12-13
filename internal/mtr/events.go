package mtr

type EventType int

const (
	EventTypeHopUpdated EventType = iota + 1
	EventTypeRoundCompleted
	EventTypeDone
	EventTypeError
)

type Event struct {
	Type  EventType
	TTL   int
	Round int
	Err   error
}
