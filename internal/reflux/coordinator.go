package reflux

import (
	"errors"
	"github.com/google/uuid"
	"github.com/wyw14/solo-039/internal/model"
	"sync"
)

type Coordinator struct {
	mu       sync.Mutex
	capacity float64
	used     float64
	states   map[string]*State
	leases   map[uuid.UUID]model.Lease
}

func NewCoordinator(capacity float64) *Coordinator {
	return &Coordinator{capacity: capacity, states: make(map[string]*State), leases: make(map[uuid.UUID]model.Lease)}
}

func (c *Coordinator) Register(name string, state *State) {
	c.mu.Lock()
	c.states[name] = state
	c.mu.Unlock()
}

func (c *Coordinator) Acquire(name string, amount float64, fence uint64) (model.Lease, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if amount <= 0 || c.used+amount > c.capacity {
		return model.Lease{}, errors.New("reflux capacity unavailable")
	}
	lease := model.Lease{ID: model.NewCampaign(name).ID, Owner: name, Capacity: amount, Fence: fence, Active: true}
	c.used += amount
	c.leases[lease.ID] = lease
	if state := c.states[name]; state != nil {
		state.SetLease(lease)
	}
	return lease, nil
}

// Release returns capacity exactly once per outstanding lease. Duplicate
// releases and stale-generation leases are ignored so they cannot create
// capacity that was never returned.
func (c *Coordinator) Release(lease model.Lease) {
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
	if state := c.states[lease.Owner]; state != nil && state.Snapshot().Lease.ID == lease.ID {
		released := lease
		released.Active = false
		state.SetLease(released)
	}
}

func (c *Coordinator) Used() float64 { c.mu.Lock(); defer c.mu.Unlock(); return c.used }
