package types

type Action[V Hashable[H], H Hash, A Addr] interface {
	IsTendermintAction()
}
