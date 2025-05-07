package tendermint

// Todo: add doc on WAL msgs
// The purpose of the WAL is to record any event that may result in a state change.
// These events fall into the following categories:
// 1. Incoming messages. We do not need to store outgoing messages.
// 2. When we propose a value.
// 3. When a timeout is triggered (not when it is scheduled).
// The purpose of the WAL is so that if the node crashes, we can recover the exact state we were in before the crash.
// Importantly we only store a WAL entry if it has not been seen before.
// When replaying the WAL, the goal is to reproduce the exact internal state prior to a crash —
// no messages should be broadcast, and no side effects should occur during replay.

type IsWALMsg interface {
	msgType() MessageType
}
