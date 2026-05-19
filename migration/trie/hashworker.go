package trie

import (
	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
	"github.com/NethermindEth/juno/core/trie2/trieutils"
	"github.com/NethermindEth/juno/db"
)

type edgeHashJob struct {
	leftChildHash, rightChildHash felt.Felt
	leftSeg, rightSeg             trieutils.Path
	parentPath                    trieutils.Path
}

type inFlightBatch struct {
	jobs    []edgeHashJob
	results []felt.Felt
	done    <-chan struct{}
}

type hashScheduler struct {
	hashFn   crypto.HashFn
	parallel bool
	bucket   db.Bucket
	owner    felt.Address
	pool     *hashWorkerPool

	jobs        []edgeHashJob
	altJobs     []edgeHashJob
	results     []felt.Felt
	inFlightBuf inFlightBatch
	hasInFlight bool
}

func newHashScheduler(
	hashFn crypto.HashFn,
	parallel bool,
	bucket db.Bucket,
	owner felt.Address,
	pool *hashWorkerPool,
) *hashScheduler {
	s := &hashScheduler{
		hashFn:   hashFn,
		parallel: parallel,
		bucket:   bucket,
		owner:    owner,
		pool:     pool,
	}
	if parallel {
		s.jobs = make([]edgeHashJob, 0, parallelHashBatchSize)
		s.altJobs = make([]edgeHashJob, 0, parallelHashBatchSize)
		s.results = make([]felt.Felt, 2*parallelHashBatchSize)
	}
	return s
}

func (s *hashScheduler) schedule(job *edgeHashJob, batch db.Batch) error {
	if !s.parallel {
		leftEdge := computeEdgeHash(&job.leftChildHash, &job.leftSeg, s.hashFn)
		rightEdge := computeEdgeHash(&job.rightChildHash, &job.rightSeg, s.hashFn)
		return s.writeBinaryAndEdges(job, &leftEdge, &rightEdge, batch)
	}
	s.jobs = append(s.jobs, *job)
	if len(s.jobs) >= parallelHashBatchSize {
		return s.fire(batch)
	}
	return nil
}

func (s *hashScheduler) fire(batch db.Batch) error {
	if err := s.drainInFlight(batch); err != nil {
		return err
	}
	results := s.results[:2*len(s.jobs)]
	s.inFlightBuf = inFlightBatch{
		jobs:    s.jobs,
		results: results,
		done:    s.pool.submit(s.hashFn, s.jobs, results),
	}
	s.hasInFlight = true
	s.jobs, s.altJobs = s.altJobs[:0], s.jobs
	return nil
}

func (s *hashScheduler) drainInFlight(batch db.Batch) error {
	if !s.hasInFlight {
		return nil
	}
	<-s.inFlightBuf.done
	for i := range s.inFlightBuf.jobs {
		err := s.writeBinaryAndEdges(
			&s.inFlightBuf.jobs[i],
			&s.inFlightBuf.results[2*i],
			&s.inFlightBuf.results[2*i+1],
			batch,
		)
		if err != nil {
			return err
		}
	}
	s.hasInFlight = false
	return nil
}

func (s *hashScheduler) sync(batch db.Batch) error {
	if !s.parallel {
		return nil
	}
	if err := s.drainInFlight(batch); err != nil {
		return err
	}
	if len(s.jobs) > 0 {
		results := s.results[:2*len(s.jobs)]
		<-s.pool.submit(s.hashFn, s.jobs, results)
		for i := range s.jobs {
			if err := s.writeBinaryAndEdges(
				&s.jobs[i],
				&results[2*i],
				&results[2*i+1],
				batch,
			); err != nil {
				return err
			}
		}
		s.jobs = s.jobs[:0]
	}
	return nil
}

func (s *hashScheduler) writeBinaryAndEdges(
	job *edgeHashJob,
	leftEdge,
	rightEdge *felt.Felt,
	batch db.Batch,
) error {
	var buf [trieutils.MaxNodeKeySize + binaryNodeBlobSize]byte
	keyLen := trieutils.EncodeNodeKey(buf[:], s.bucket, &s.owner, &job.parentPath, false)
	blob := encodeBinaryNode(leftEdge, rightEdge)
	copy(buf[keyLen:], blob[:])
	if err := batch.Put(buf[:keyLen], buf[keyLen:keyLen+binaryNodeBlobSize]); err != nil {
		return err
	}

	if job.leftSeg.Len() > 0 {
		var leftEdgePath trieutils.Path
		leftEdgePath.AppendBit(&job.parentPath, 0)
		var ebuf [trieutils.MaxNodeKeySize + edgeNodeMaxSize]byte
		kl := trieutils.EncodeNodeKey(ebuf[:], s.bucket, &s.owner, &leftEdgePath, false)
		edgeBlob := encodeEdgeNodeInto(ebuf[kl:], &job.leftChildHash, &job.leftSeg)
		if err := batch.Put(ebuf[:kl], ebuf[kl:kl+edgeBlob]); err != nil {
			return err
		}
	}

	if job.rightSeg.Len() > 0 {
		var rightEdgePath trieutils.Path
		rightEdgePath.AppendBit(&job.parentPath, 1)
		var ebuf [trieutils.MaxNodeKeySize + edgeNodeMaxSize]byte
		kl := trieutils.EncodeNodeKey(ebuf[:], s.bucket, &s.owner, &rightEdgePath, false)
		edgeBlob := encodeEdgeNodeInto(ebuf[kl:], &job.rightChildHash, &job.rightSeg)
		if err := batch.Put(ebuf[:kl], ebuf[kl:kl+edgeBlob]); err != nil {
			return err
		}
	}
	return nil
}
