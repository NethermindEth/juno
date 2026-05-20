package trie

import (
	"sync"

	"github.com/NethermindEth/juno/core/crypto"
	"github.com/NethermindEth/juno/core/felt"
)

type hashWork struct {
	hashFn  crypto.HashFn
	jobs    []edgeHashJob
	results []felt.Felt
	wg      *sync.WaitGroup
}

type hashWorkerPool struct {
	work chan hashWork
	n    int
}

func newHashWorkerPool() *hashWorkerPool {
	p := &hashWorkerPool{
		work: make(chan hashWork, IngestorCount*2),
		n:    IngestorCount,
	}
	for range IngestorCount {
		go func() {
			for w := range p.work {
				for i := range w.jobs {
					w.results[2*i] = computeEdgeHash(&w.jobs[i].leftChildHash, &w.jobs[i].leftSeg, w.hashFn)
					w.results[2*i+1] = computeEdgeHash(&w.jobs[i].rightChildHash, &w.jobs[i].rightSeg, w.hashFn)
				}
				w.wg.Done()
			}
		}()
	}
	return p
}

func (p *hashWorkerPool) submit(
	hashFn crypto.HashFn,
	jobs []edgeHashJob,
	results []felt.Felt,
) <-chan struct{} {
	done := make(chan struct{})
	go func() {
		var wg sync.WaitGroup
		chunkSize := max(1, (len(jobs)+p.n-1)/p.n)
		for i := 0; i < len(jobs); i += chunkSize {
			end := min(i+chunkSize, len(jobs))
			wg.Add(1)
			p.work <- hashWork{hashFn, jobs[i:end], results[2*i : 2*end], &wg}
		}
		wg.Wait()
		close(done)
	}()
	return done
}

func (p *hashWorkerPool) close() { close(p.work) }
