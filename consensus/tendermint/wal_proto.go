package tendermint

// WALToProto takes a WAL message and return a proto walMessage and error
func WALToProto(msg WALMessage) (*tmcons.WALMessage, error) {
	panic("to implement encoding each message type")
}

// WALFromProto takes a proto wal message and return a consensus walMessage and error
func WALFromProto(msg WALMessage) (*tmcons.WALMessage, error) {
	panic("to implement encoding each message type")
}
