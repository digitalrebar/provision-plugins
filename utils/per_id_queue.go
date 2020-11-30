package utils

import (
	"context"
	"fmt"
	"sync"

	"github.com/digitalrebar/logger"
)

type PerIdQueue struct {
	Capacity int
	ctx      context.Context
	mux      *sync.Mutex
	queues   map[string]chan func()
}

func NewQueues(ctx context.Context, limit int) *PerIdQueue {
	return &PerIdQueue{
		ctx:      ctx,
		Capacity: limit,
		mux:      &sync.Mutex{},
		queues:   map[string]chan func(){},
	}
}

func (pmq *PerIdQueue) Add(id string, l logger.Logger, req func()) error {
	pmq.mux.Lock()
	defer pmq.mux.Unlock()
	ch, ok := pmq.queues[id]
	if !ok {
		ch = make(chan func(), pmq.Capacity)
		pmq.queues[id] = ch
		go func() {
			for {
				select {
				case <-pmq.ctx.Done():
					return
				case fn, ok := <-ch:
					if !ok {
						return
					}
					fn()
				}
			}
		}()
	}
	if len(ch) == pmq.Capacity {
		return fmt.Errorf("Queued action for %s: overloaded, %d outstanding callbacks in flight", id, pmq.Capacity)
	}
	ch <- func() {
		defer func() {
			if x := recover(); x != nil {
				l.Errorf("panic recovered: %v", x)
			}
			pmq.mux.Lock()
			defer pmq.mux.Unlock()
			if len(ch) == 0 {
				close(ch)
				delete(pmq.queues, id)
			}
		}()
		req()
	}
	return nil
}
