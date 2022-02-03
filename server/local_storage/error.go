package local_storage

type error_code uint

const (
	// Couldn't lock the file for storing the data.
	ErrStoreLockFailed error_code = iota
	// Trying to store a duplicated data.
	ErrDuplicatedStore
	// Couldn't save the data.
	ErrStoreFailed
	// Couldn't lock the file for reading the data.
	ErrGetLockFailed
	// Couldn't read any local data.
	ErrGetFailed
	// Couldn't find any local data for reading.
	ErrGetEmpty
	// Couldn't remove the local data.
	ErrRemoveFailed
	// Wait timed out.
	ErrTimedOut
	// The local storage was closed.
	ErrStoreClosed
)

func (e error_code) Error() string {
	switch e {
	case ErrStoreLockFailed:
		return "Couldn't lock the file for storing the data."
	case ErrDuplicatedStore:
		return "Trying to store a duplicated data."
	case ErrStoreFailed:
		return "Couldn't save the data."
	case ErrGetLockFailed:
		return "Couldn't lock the file for reading the data."
	case ErrGetFailed:
		return "Couldn't read any local data."
	case ErrGetEmpty:
		return "Couldn't find any file for reading."
	case ErrRemoveFailed:
		return "Couldn't remove the local data."
	case ErrTimedOut:
		return "Wait timed out."
	case ErrStoreClosed:
		return "The local storage was closed."
	default:
		return "Invalid local_storage error."
	}
}
