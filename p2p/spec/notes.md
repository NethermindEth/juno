* Goals: consensus, scale, validator-fullnode separation, cleanup
* whenever 'bytes' is used to encode large numbers (felts), big endian is used, without storing leading zeros in the number as bytes (so 0x11 is stored as [0x11] not [0x00, 0x00,...,0x11]
* requests are responded to with a stream of messages that are varint message delimited.
* When a stream returnes several logical objects, each in several messages, then each object messages should end with a Fin message
* Responses to events, receipts and transactions also include block hash for cases of reorg.
* request_id is handled at the protocol level, specifically to support multiplexing
* number of messages (especially repeated ones) is capped for ddos
* Getting transactions/events is separate from getting blocks
* Protocols: blocks (headers+bodies or separate?), transactions, events, receipts, mempool. States? fastsync?
* Using Merkle trees so can prove a range of values (in the future). Also, a light node can decommit individual values (e.g. events)
** TBD: can save on the internal merkle nodes by having one where a leaf is the tx data, events and receipts, each hashed separately. But then getting a part of a leaf will require sending the other parts' hashes.
** Events are separate from receipt
* Proof and signatures are required to be returned only for blocks after the last l1 state update
* TBD: consensus messages (tx streaming, proofs separate)
* TBD: reverted transactions
* TBD: stark friendly hashes or not (calculate in os? so light nodes don't trust the consensus)
* TBD: "pubsub" mechanism where a node subscribes to getting push messages instead of GetXXX.
** GetXXX messages will need to be supported for network issues / failures
* Consensus: send (Transactions*, StateDiff*)* Proposal. The state diffs allows paralelization as the transactions following it can be executed from that state in parallel to whatever is already executing
* Assuming L1 state update will have block header hash and number.
* tx calldata limited to 30K

