package health

type Error struct {
	Code    int
	Message string
}

func (s Error) Error() string {
	return s.Message
}

var (
	ErrorNodeNotSyncing = Error{
		Code:    10,
		Message: "Node is not syncing",
	}
	ErrorVMNotRunning = Error{
		Code:    11,
		Message: "Cairo VM is not running",
	}
	ErrorUnHealthy = Error{
		Code:    12,
		Message: "Node is not healthy",
	}
)
