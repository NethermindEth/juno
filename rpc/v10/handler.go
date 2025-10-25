package rpcv10

type Handler struct{}

func New() *Handler {
	return &Handler{}
}

func (h *Handler) SpecVersion() (string, error) {
	return "0.10.0-rc.1", nil
}
