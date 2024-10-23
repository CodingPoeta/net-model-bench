package jnet

import (
	"sync"
)

type workerEntry struct {
	ch chan *request
}

type WorkerPool struct {
	pool  *sync.Pool
	ready []*workerEntry
	mu    sync.Mutex
}

func NewWorkerPool() *WorkerPool {
	return &WorkerPool{
		pool: &sync.Pool{
			New: func() any {
				return &workerEntry{
					ch: make(chan *request),
				}
			},
		},
		ready: make([]*workerEntry, 0),
	}
}

func (p *WorkerPool) WorkFunc(ent *workerEntry, req *request) {
	req.backend.processRequest(req)
	ch := ent.ch
	p.mu.Lock()
	p.ready = append(p.ready, ent)
	p.mu.Unlock()
	for req := range ch {
		if req == nil {
			return
		}
		req.backend.processRequest(req)
		p.mu.Lock()
		p.ready = append(p.ready, ent)
		p.mu.Unlock()
	}
}

func (p *WorkerPool) Submit(req *request) {
	p.mu.Lock()
	if len(p.ready) > 0 {
		ent := p.ready[len(p.ready)-1]
		p.ready = p.ready[:len(p.ready)-1]
		p.mu.Unlock()
		ent.ch <- req
		return
	}
	p.mu.Unlock()
	go func() {
		ent := p.pool.Get().(*workerEntry)
		p.WorkFunc(ent, req)
		p.pool.Put(ent)
	}()
}
