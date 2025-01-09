package trie2

import (
	"bytes"
	"sync"
)

var bufferPool = sync.Pool{
	New: func() interface{} {
		return new(bytes.Buffer)
	},
}

func (n *binaryNode) write(buf *bytes.Buffer) error {
	if err := n.children[0].write(buf); err != nil {
		return err
	}

	if err := n.children[1].write(buf); err != nil {
		return err
	}

	return nil
}

func (n *edgeNode) write(buf *bytes.Buffer) error {
	if _, err := n.path.Write(buf); err != nil {
		return err
	}

	if err := n.child.write(buf); err != nil {
		return err
	}

	return nil
}

func (n hashNode) write(buf *bytes.Buffer) error {
	if _, err := buf.Write(n.Felt.Marshal()); err != nil {
		return err
	}

	return nil
}

func (n valueNode) write(buf *bytes.Buffer) error {
	if _, err := buf.Write(n.Felt.Marshal()); err != nil {
		return err
	}

	return nil
}

func nodeToBytes(n node) []byte {
	buf := bufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer func() {
		buf.Reset()
		bufferPool.Put(buf)
	}()

	if err := n.write(buf); err != nil {
		panic(err)
	}
	return buf.Bytes()
}
