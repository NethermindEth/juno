package health

type Error struct {
	Code    int
	Message string
}

func (s Error) Error() string {
	return s.Message
}

var (
	NodeNotSyncing = Error{
		Code:    10,
		Message: "Node is not syncing",
	}
	VMNotRunning = Error{
		Code:    20,
		Message: "Cairo VM is not running",
	}
	UnHealthy = Error{
		Code:    30,
		Message: "Node is not healthy",
	}
)
