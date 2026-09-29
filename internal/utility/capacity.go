package utility

import (
	"errors"
	"github.com/google/uuid"
	"github.com/wyw14/solo-039/internal/model"
	"sync"
)

type Capacity struct {
	mu     sync.Mutex
	total  float64
	used   float64
	leases map[uuid.UUID]model.Lease
}

func NewCapacity(total float64) *Capacity {
	return &Capacity{total: total, leases: make(map[uuid.UUID]model.Lease)}
}

func (c *Capacity) Reserve(owner string, amount float64, fence uint64) (model.Lease, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if amount <= 0 || c.used+amount > c.total {
		return model.Lease{}, errors.New("capacity unavailable")
	}
	lease := model.Lease{ID: uuid.New(), Owner: owner, Capacity: amount, Fence: fence, Active: true}
	c.used += amount
	c.leases[lease.ID] = lease
	return lease, nil
}

// Release returns capacity exactly once per outstanding lease. Duplicate
// releases and stale-generation leases are ignored so they cannot create
// capacity that was never returned.
func (c *Capacity) Release(lease model.Lease) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !lease.Active {
		return
	}
	outstanding, ok := c.leases[lease.ID]
	if !ok || outstanding.Fence != lease.Fence {
		return
	}
	delete(c.leases, lease.ID)
	c.used -= outstanding.Capacity
	if c.used < 0 {
		c.used = 0
	}
}

func (c *Capacity) Used() float64 { c.mu.Lock(); defer c.mu.Unlock(); return c.used }
