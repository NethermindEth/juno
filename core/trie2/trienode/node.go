package trienode

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
)

var (
	_ Node = (*BinaryNode)(nil)
	_ Node = (*EdgeNode)(nil)
	_ Node = (*HashNode)(nil)
	_ Node = (*ValueNode)(nil)
)

type Node interface {
	Hash(crypto.HashFn) felt.Felt
	Cache() (*HashNode, bool)
	Write(*bytes.Buffer) error
	String() string
}

// Represents a binary branch node in the trie with two children
type BinaryNode struct {
	Children [2]Node // 0 = left, 1 = right
	Flags    nodeFlag
}

func (n *BinaryNode) Hash(hf crypto.HashFn) felt.Felt {
	leftHash := n.Left().Hash(hf)
	rightHash := n.Right().Hash(hf)

	return hf(&leftHash, &rightHash)
}

func (n *BinaryNode) Cache() (*HashNode, bool) { return n.Flags.Hash, n.Flags.Dirty }

func (n *BinaryNode) String() string {
	var left, right string
	if n.Left() != nil {
		left = n.Left().String()
	}
	if n.Right() != nil {
		right = n.Right().String()
	}
	return fmt.Sprintf("Binary[\n  left: %s\n  right: %s\n]",
		indent(left),
		indent(right))
}

func (n *BinaryNode) Copy() *BinaryNode { cpy := *n; return &cpy }

func (n *BinaryNode) Left() Node { return n.Children[0] }

func (n *BinaryNode) Right() Node { return n.Children[1] }

// Represents a path-compressed node that stores a path segment
// and a single child node
type EdgeNode struct {
	Child Node            // The child node at the end of the path
	Path  *trieutils.Path // The compressed path segment
	Flags nodeFlag
}

func (n *EdgeNode) Hash(hf crypto.HashFn) felt.Felt {
	var length [32]byte
	length[31] = n.Path.Len()
	pathFelt := n.Path.Felt()
	lengthFelt := felt.FromBytes[felt.Felt](length[:])

	childHash := n.Child.Hash(hf)
	innerHash := hf(&childHash, &pathFelt)

	var res felt.Felt
	res.Add(&innerHash, &lengthFelt)
	return res
}

func (n *EdgeNode) Cache() (*HashNode, bool) { return n.Flags.Hash, n.Flags.Dirty }

func (n *EdgeNode) String() string {
	var child string
	if n.Child != nil {
		child = n.Child.String()
	}
	return fmt.Sprintf("Edge{\n  Path: %s\n  Child: %s\n}",
		n.Path.String(),
		indent(child))
}

func (n *EdgeNode) Copy() *EdgeNode { cpy := *n; return &cpy }

func (n *EdgeNode) PathMatches(key *trieutils.Path) bool {
	return n.Path.EqualMSBs(key)
}

// Returns the common bits between the current node and the given key, starting from the most significant bit
func (n *EdgeNode) CommonPath(key *trieutils.Path) trieutils.Path {
	var commonPath trieutils.Path
	commonPath.CommonMSBs(n.Path, key)
	return commonPath
}

// Represents a node that only contains a hash reference to another node
type HashNode felt.Felt

func (n *HashNode) Hash(crypto.HashFn) felt.Felt { return felt.Felt(*n) }

func (n *HashNode) Cache() (*HashNode, bool) { return nil, true }

func (n HashNode) String() string {
	f := felt.Felt(n)
	return fmt.Sprintf("Hash(%s)", f.String())
}

// Represents a leaf node that stores an actual value in the trie
type ValueNode felt.Felt

func (n *ValueNode) Hash(crypto.HashFn) felt.Felt { return felt.Felt(*n) }

func (n *ValueNode) Cache() (*HashNode, bool) { return nil, true }

func (n ValueNode) String() string {
	f := felt.Felt(n)
	return fmt.Sprintf("Value(%s)", f.String())
}

// Used when collapsing internal trie nodes for hashing, since unset children need to be hashed correctly
var NilValueNode = (*ValueNode)(&felt.Felt{})

// nodeFlag tracks a trie node's state by caching its hash and marking if it's been modified.
// This optimization prevents redundant hash calculations. When node is modified or created, the flag is marked as dirty.
// When the node is read from the database, the hash is cached and the node is marked as clean.
type nodeFlag struct {
	Hash  *HashNode // The cached hash of the node
	Dirty bool      // Whether the node has been modified
}

// Creates a new node flag and marks the node as dirty
func NewNodeFlag() nodeFlag { return nodeFlag{Dirty: true} }

// Helper function to indent each line of a string
func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
