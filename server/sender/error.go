package sender

type error_code uint

const (
	// Invalid input.
	ErrInvalidInput error_code = iota
	// Failed to send the message.
	ErrSendFailed
)

func (e error_code) Error() string {
	switch e {
	case ErrInvalidInput:
		return "Invalid input."
	case ErrSendFailed:
		return "Failed to send the message."
	default:
		return "Invalid local_storage error."
	}
}
