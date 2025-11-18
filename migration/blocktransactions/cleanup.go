package blocktransactions

import (
	"errors"
	"io"

	"github.com/NethermindEth/juno/db"
	"github.com/sourcegraph/conc/pool"
)

type cleanup struct {
	closers *pool.ErrorPool
	err     error
}

func newCleanup() cleanup {
	return cleanup{
		closers: pool.New().WithErrors(),
		err:     nil,
	}
}

func (c *cleanup) CloseBatch(batch db.Batch) {
	c.closers.Go(batch.(io.Closer).Close)
}

func (c *cleanup) AddError(err error) {
	c.err = errors.Join(c.err, err)
}

func (c *cleanup) Wait() error {
	return errors.Join(c.err, c.closers.Wait())
}
