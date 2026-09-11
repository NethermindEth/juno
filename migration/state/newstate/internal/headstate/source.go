package headstate

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"iter"

	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/db"
)

type pendingContract struct {
	addr      felt.Address
	classHash felt.Felt
	nonce     felt.Felt
	height    uint64
}

// cursor walks one address-keyed bucket in ascending address order.
type cursor struct {
	it     db.Iterator
	prefix []byte
	bucket db.Bucket
	addr   []byte // address suffix of the current key, nil once exhausted
	err    error
}

func newCursor(r db.KeyValueReader, bucket db.Bucket) (*cursor, error) {
	prefix := bucket.Key()
	it, err := r.NewIterator(prefix, true)
	if err != nil {
		return nil, fmt.Errorf("opening iterator for bucket %v: %w", bucket, err)
	}
	c := &cursor{it: it, prefix: prefix, bucket: bucket}
	c.set(it.First())
	return c, c.err
}

func (c *cursor) set(valid bool) {
	if !valid {
		c.addr = nil
		return
	}
	key := c.it.Key()
	if len(key) != len(c.prefix)+felt.Bytes {
		c.err = fmt.Errorf(
			"malformed %v key: len %d, want %d",
			c.bucket, len(key), len(c.prefix)+felt.Bytes,
		)
		c.addr = nil
		return
	}
	c.addr = key[len(c.prefix):]
}

func (c *cursor) next() { c.set(c.it.Next()) }

// advanceTo moves to the first address >= target, reporting an exact hit. It
// never moves backwards, which is what keeps the pass sequential.
func (c *cursor) advanceTo(target []byte) bool {
	for c.addr != nil && bytes.Compare(c.addr, target) < 0 {
		c.next()
	}
	return c.addr != nil && bytes.Equal(c.addr, target)
}

// resolve assembles the record for the address the driver sits on.
func resolve(driver, nonces, heights *cursor, addr []byte) (*pendingContract, error) {
	rec := &pendingContract{addr: felt.FromBytes[felt.Address](addr)}

	raw, err := driver.it.UncopiedValue()
	if err != nil {
		return nil, fmt.Errorf("reading class hash for %s: %w", &rec.addr, err)
	}
	rec.classHash.SetBytes(raw)

	// A missing nonce means the contract was never updated.
	if nonces.advanceTo(addr) {
		raw, err := nonces.it.UncopiedValue()
		if err != nil {
			return nil, fmt.Errorf("reading nonce for %s: %w", &rec.addr, err)
		}
		rec.nonce.SetBytes(raw)
	}

	if !heights.advanceTo(addr) {
		return nil, fmt.Errorf("no deployment height for %s", &rec.addr)
	}
	raw, err = heights.it.UncopiedValue()
	if err != nil {
		return nil, fmt.Errorf("reading deployment height for %s: %w", &rec.addr, err)
	}
	rec.height = binary.BigEndian.Uint64(raw)

	return rec, errors.Join(driver.err, nonces.err, heights.err)
}

// pendingContracts walks the deprecated buckets and Contract in lockstep. All
// four are keyed by address, so one sequential pass replaces every point read.
// ContractClassHash drives: it defines the contract set.
func pendingContracts(r db.KeyValueReader) (iter.Seq[*pendingContract], func() error) {
	var iterErr error

	seq := func(yield func(*pendingContract) bool) {
		cursors := make([]*cursor, 0, 4)
		defer func() {
			for _, c := range cursors {
				c.it.Close()
			}
		}()

		for _, bucket := range []db.Bucket{
			db.ContractClassHash,
			db.ContractNonce,
			db.ContractDeploymentHeight,
			db.Contract,
		} {
			c, err := newCursor(r, bucket)
			if err != nil {
				iterErr = err
				return
			}
			cursors = append(cursors, c)
		}
		driver, nonces, heights, migrated := cursors[0], cursors[1], cursors[2], cursors[3]

		for driver.addr != nil {
			addr := driver.addr

			if migrated.advanceTo(addr) {
				driver.next()
				continue
			}

			rec, err := resolve(driver, nonces, heights, addr)
			if err != nil {
				iterErr = err
				return
			}
			if !yield(rec) {
				return
			}
			driver.next()
		}

		iterErr = errors.Join(driver.err, nonces.err, heights.err, migrated.err)
	}

	return seq, func() error { return iterErr }
}
