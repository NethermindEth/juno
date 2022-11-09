package db

// Address: encoded account
const State = "State"

// Address: bytecode
const Code = "Code"

// Prefix: node
const TrieAccount = "TrieAccount"
// Address + Prefix: node
const TrieStorage = "TrieStorage"

// blockNum: Address: original value
const ChangeSetAccount = "ChangeSetAccount"
// blockNum: Address + slot: original value
const ChangeSetStorage = "ChangeSetStorage"
