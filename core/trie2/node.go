package trie2

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

var (
	_ node = (*binaryNode)(nil)
	_ node = (*edgeNode)(nil)
	_ node = (*hashNode)(nil)
	_ node = (*valueNode)(nil)
)

type node interface {
	hash(crypto.HashFn) *felt.Felt // TODO(weiihann): return felt value instead of pointers
	cache() (*hashNode, bool)
	write(*bytes.Buffer) error
	String() string
}

type (
	binaryNode struct {
		children [2]node // 0 = left, 1 = right
		flags    nodeFlag
	}
	edgeNode struct {
		child node
		path  *Path
		flags nodeFlag
	}
	hashNode  struct{ felt.Felt }
	valueNode struct{ felt.Felt }
)

type nodeFlag struct {
	hash  *hashNode
	dirty bool
}

func newFlag() nodeFlag { return nodeFlag{dirty: true} }

func (n *binaryNode) hash(hf crypto.HashFn) *felt.Felt {
	return hf(n.children[0].hash(hf), n.children[1].hash(hf))
}

func (n *edgeNode) hash(hf crypto.HashFn) *felt.Felt {
	var length [32]byte
	length[31] = n.path.Len()
	pathFelt := n.path.Felt()
	lengthFelt := new(felt.Felt).SetBytes(length[:])
	return new(felt.Felt).Add(hf(n.child.hash(hf), &pathFelt), lengthFelt)
}

func (n *hashNode) hash(crypto.HashFn) *felt.Felt  { return &n.Felt }
func (n *valueNode) hash(crypto.HashFn) *felt.Felt { return &n.Felt }

func (n *binaryNode) cache() (*hashNode, bool) { return n.flags.hash, n.flags.dirty }
func (n *edgeNode) cache() (*hashNode, bool)   { return n.flags.hash, n.flags.dirty }
func (n *hashNode) cache() (*hashNode, bool)   { return nil, true }
func (n *valueNode) cache() (*hashNode, bool)  { return nil, true }

func (n *binaryNode) String() string {
	var left, right string
	if n.children[0] != nil {
		left = n.children[0].String()
	}
	if n.children[1] != nil {
		right = n.children[1].String()
	}
	return fmt.Sprintf("Binary[\n  left: %s\n  right: %s\n]",
		indent(left),
		indent(right))
}

func (n *edgeNode) String() string {
	var child string
	if n.child != nil {
		child = n.child.String()
	}
	return fmt.Sprintf("Edge{\n  path: %s\n  child: %s\n}",
		n.path.String(),
		indent(child))
}

func (n hashNode) String() string {
	return fmt.Sprintf("Hash(%s)", n.Felt.String())
}

func (n valueNode) String() string {
	return fmt.Sprintf("Value(%s)", n.Felt.String())
}

// TODO(weiihann): check if we want to return a pointer or a value
func (n *binaryNode) copy() *binaryNode { cpy := *n; return &cpy }
func (n *edgeNode) copy() *edgeNode     { cpy := *n; return &cpy }

func (n *edgeNode) pathMatches(key *Path) bool {
	return n.path.EqualMSBs(key)
}

// Returns the common bits between the current node and the given key, starting from the most significant bit
func (n *edgeNode) commonPath(key *Path) Path {
	var commonPath Path
	commonPath.CommonMSBs(n.path, key)
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
