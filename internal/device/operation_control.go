package device

import (
	"context"
	"sync"
)

type deviceOperationControl struct {
	mutex      sync.Mutex
	cancel     context.CancelFunc
	generation uint64
}

func (c *deviceOperationControl) begin() (context.Context, func()) {
	ctx, cancel := context.WithCancel(context.Background())
	c.mutex.Lock()
	if c.cancel != nil {
		c.cancel()
	}
	c.generation++
	generation := c.generation
	c.cancel = cancel
	c.mutex.Unlock()

	return ctx, func() {
		cancel()
		c.mutex.Lock()
		if c.generation == generation {
			c.cancel = nil
		}
		c.mutex.Unlock()
	}
}

func (c *deviceOperationControl) cancelActive() {
	c.mutex.Lock()
	cancel := c.cancel
	c.mutex.Unlock()
	if cancel != nil {
		cancel()
	}
}
