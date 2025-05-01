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

type (
	// Represents a binary branch node in the trie with two children
	BinaryNode struct {
		Children [2]Node // 0 = left, 1 = right
		Flags    NodeFlag
	}
	// Represents a path-compressed node that stores a path segment
	// and a single child node
	EdgeNode struct {
		Child Node            // The child node at the end of the path
		Path  *trieutils.Path // The compressed path segment
		Flags NodeFlag
	}
	// Represents a node that only contains a hash reference to another node
	HashNode struct{ felt.Felt }
	// Represents a leaf node that stores an actual value in the trie
	ValueNode struct{ felt.Felt }
)

// Used when collapsing internal trie nodes for hashing, since unset children need to be hashed correctly
var NilValueNode = &ValueNode{felt.Felt{}}

type NodeFlag struct {
	Hash  *HashNode
	Dirty bool
}

func NewNodeFlag() NodeFlag { return NodeFlag{Dirty: true} }

func (n *BinaryNode) Hash(hf crypto.HashFn) felt.Felt {
	leftHash := n.Children[0].Hash(hf)
	rightHash := n.Children[1].Hash(hf)
	res := hf(&leftHash, &rightHash)
	return *res
}

func (n *EdgeNode) Hash(hf crypto.HashFn) felt.Felt {
	var length [32]byte
	length[31] = n.Path.Len()
	pathFelt := n.Path.Felt()
	lengthFelt := new(felt.Felt).SetBytes(length[:])

	childHash := n.Child.Hash(hf)
	innerHash := hf(&childHash, &pathFelt)

	var res felt.Felt
	res.Add(innerHash, lengthFelt)
	return res
}

func (n *HashNode) Hash(crypto.HashFn) felt.Felt  { return n.Felt }
func (n *ValueNode) Hash(crypto.HashFn) felt.Felt { return n.Felt }

func (n *BinaryNode) Cache() (*HashNode, bool) { return n.Flags.Hash, n.Flags.Dirty }
func (n *EdgeNode) Cache() (*HashNode, bool)   { return n.Flags.Hash, n.Flags.Dirty }
func (n *HashNode) Cache() (*HashNode, bool)   { return nil, true }
func (n *ValueNode) Cache() (*HashNode, bool)  { return nil, true }

func (n *BinaryNode) String() string {
	var left, right string
	if n.Children[0] != nil {
		left = n.Children[0].String()
	}
	if n.Children[1] != nil {
		right = n.Children[1].String()
	}
	return fmt.Sprintf("Binary[\n  left: %s\n  right: %s\n]",
		indent(left),
		indent(right))
}

func (n *EdgeNode) String() string {
	var child string
	if n.Child != nil {
		child = n.Child.String()
	}
	return fmt.Sprintf("Edge{\n  Path: %s\n  Child: %s\n}",
		n.Path.String(),
		indent(child))
}

func (n HashNode) String() string {
	return fmt.Sprintf("Hash(%s)", n.Felt.String())
}

func (n ValueNode) String() string {
	return fmt.Sprintf("Value(%s)", n.Felt.String())
}

func (n *BinaryNode) Copy() *BinaryNode { cpy := *n; return &cpy }
func (n *EdgeNode) Copy() *EdgeNode     { cpy := *n; return &cpy }

func (n *EdgeNode) PathMatches(key *trieutils.Path) bool {
	return n.Path.EqualMSBs(key)
}

// Returns the common bits between the current node and the given key, starting from the most significant bit
func (n *EdgeNode) CommonPath(key *trieutils.Path) trieutils.Path {
	var commonPath trieutils.Path
	commonPath.CommonMSBs(n.Path, key)
	return commonPath
}

// Helper function to indent each line of a string
func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		lines[i] = "  " + line
	}
	return strings.Join(lines, "\n")
}
