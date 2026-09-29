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
	active   map[uuid.UUID]model.Lease
	fences   map[string]uint64
	states   map[string]*State
}

func NewCoordinator(capacity float64) *Coordinator {
	return &Coordinator{
		capacity: capacity,
		active:   make(map[uuid.UUID]model.Lease),
		fences:   make(map[string]uint64),
		states:   make(map[string]*State),
	}
}

func (c *Coordinator) Register(name string, state *State) {
	c.mu.Lock()
	c.states[name] = state
	c.mu.Unlock()
}

func (c *Coordinator) Acquire(name string, amount float64, fence uint64) (model.Lease, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if amount <= 0 {
		return model.Lease{}, errors.New("reflux capacity unavailable")
	}
	// A request from an older campaign generation must not consume capacity.
	if latest := c.fences[name]; fence < latest {
		return model.Lease{}, errors.New("reflux lease fence is stale")
	}
	// Defensive: never grant against a corrupted/negative used value.
	if c.used < 0 {
		c.used = 0
	}
	if c.used+amount > c.capacity {
		return model.Lease{}, errors.New("reflux capacity unavailable")
	}
	c.used += amount
	c.fences[name] = fence
	lease := model.Lease{ID: model.NewCampaign(name).ID, Owner: name, Capacity: amount, Fence: fence, Active: true}
	c.active[lease.ID] = lease
	if state := c.states[name]; state != nil {
		state.SetLease(lease)
	}
	return lease, nil
}

// Release returns a lease's capacity exactly once. Repeated calls, releases of
// unknown/already-returned lease IDs, and requests carrying an old generation's
// fence are no-ops and can never manufacture additional capacity.
func (c *Coordinator) Release(lease model.Lease) {
	c.mu.Lock()
	defer c.mu.Unlock()
	current, ok := c.active[lease.ID]
	if !ok {
		return
	}
	if current.Fence != lease.Fence || current.Owner != lease.Owner {
		return
	}
	delete(c.active, lease.ID)
	c.used -= current.Capacity
	if c.used < 0 {
		c.used = 0
	}
	released := current
	released.Active = false
	if state := c.states[current.Owner]; state != nil {
		state.SetLease(released)
	}
}

func (c *Coordinator) Used() float64 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.used
}
