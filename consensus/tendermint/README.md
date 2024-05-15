# Tendermint Implementation
This document contains the most recent design for the implementation of Tendermint for the juno client.

[visit here for more detailed explanation with diagrams](https://www.notion.so/nethermind/Tendermint-Consensus-f5f337c7a90046efbcf98fa2eafa8279?pvs=4#dabf0214f3324e5eba186fbe167cf056)

## Component
At a higher level the components
1. Tendermint core
2. Adapters
3. Misc Entities
4. Everything else


### Tendermint core
This includes:
1. Tendermint manager
2. The atomic state machine
3. Message Adapters (decided values, broadcast and receiving)
4. Message entities emitted by tendermint (values decided on, messages, [timeouts])

**Methods:**

#### Tendermint manager
This model encapsulate the entirety of the tendermint idea.
It's **required dependencies** include `two adpaters`. The `Gossip adapter` that handles transport and 
restructuring of in/out messages to tendermint. The `Decision adapter` that handles temp storage of decided
values. The paper uses an array, but on observation the algorithm only ever cares about the current value to decided
upon. So adapter that can collect all decided values/retrieve all previous ones from some store would be quite
sufficient and abstract enough.
  
A couple of optional dependencies for testing purposes include the state machine itself.
this allows for swapping the state-machine with one that can be mocked and easily tested.

**Methods:**

#### State machine
1. The prev decided upon value (optional)
2. The current value to be decided upon (optional)
3. a write only lock (required dependency)
4. State: height, round, step, lockedValue, lockedRound, validValue, validRound (optional dependency)

**Methods:**

#### Messages
Messages differ based on the adapter in question.
1. For the decision adapter. Messages contain Values (an ordered collection of transactions, proofs, and height)
2. For the Gossip adapter. Messages include the step, round, height, payload (value, id) based on the step


**Methods:**

### Adapters
These include:
1. The Gossip manager (contains and owns the input and output queue for messages)
2. The Chain manager (contains and owns the decided Values queue)

**Methods:**

### Misc Entities

**Methods:**

#### Decided Value
1. Collection of transactions
2. Proof
3. height
4. decision round

**Methods:**

#### ID Value
1. id

#### Timeouts (decided not to do this)
- ignore!

### Everything else
These include everything outside tendermint that does not yet interact with the consensus module


