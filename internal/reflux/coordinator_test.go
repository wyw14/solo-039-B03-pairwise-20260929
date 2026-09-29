package reflux

import (
	"testing"

	"github.com/google/uuid"
	"github.com/wyw14/solo-039/internal/model"
)

func TestCoordinatorBoundsSequentialCapacity(t *testing.T) {
	coordinator := NewCoordinator(1)
	if _, err := coordinator.Acquire("T-101", 0.7, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Acquire("T-102", 0.7, 1); err == nil {
		t.Fatal("second allocation exceeded capacity")
	}
	if coordinator.Used() > 1 {
		t.Fatalf("capacity exceeded: %f", coordinator.Used())
	}
}

func TestCoordinatorReleaseIsIdempotent(t *testing.T) {
	coordinator := NewCoordinator(1)
	lease, err := coordinator.Acquire("T-101", 0.6, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Acquire("T-102", 0.4, 1); err != nil {
		t.Fatal(err)
	}
	coordinator.Release(lease)
	coordinator.Release(lease)
	if got := coordinator.Used(); got != 0.4 {
		t.Fatalf("duplicate release freed capacity twice: used=%f", got)
	}
	if _, err := coordinator.Acquire("T-103", 0.7, 1); err == nil {
		t.Fatal("duplicate release manufactured capacity beyond the pump limit")
	}
}

func TestCoordinatorIgnoresStaleGenerationRelease(t *testing.T) {
	coordinator := NewCoordinator(1)
	first, err := coordinator.Acquire("T-101", 0.5, 1)
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Release(first)
	second, err := coordinator.Acquire("T-101", 0.5, 2)
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Release(first)
	if got := coordinator.Used(); got != 0.5 {
		t.Fatalf("stale-generation release freed live capacity: used=%f", got)
	}
	forged := second
	forged.Fence = 1
	coordinator.Release(forged)
	if got := coordinator.Used(); got != 0.5 {
		t.Fatalf("fenced-out release freed live capacity: used=%f", got)
	}
	coordinator.Release(second)
	if got := coordinator.Used(); got != 0 {
		t.Fatalf("valid lease was not returned exactly once: used=%f", got)
	}
}

func TestCoordinatorIgnoresUnknownLease(t *testing.T) {
	coordinator := NewCoordinator(1)
	coordinator.Release(model.Lease{ID: uuid.New(), Owner: "T-109", Capacity: 0.9, Fence: 1, Active: true})
	coordinator.Release(model.Lease{})
	if got := coordinator.Used(); got != 0 {
		t.Fatalf("unknown lease manufactured capacity: used=%f", got)
	}
}

func TestCoordinatorReleaseDeactivatesRegisteredLease(t *testing.T) {
	coordinator := NewCoordinator(1)
	state := New()
	coordinator.Register("T-101", state)
	lease, err := coordinator.Acquire("T-101", 0.5, 1)
	if err != nil {
		t.Fatal(err)
	}
	coordinator.Release(lease)
	if state.Snapshot().Lease.Active {
		t.Fatal("released lease still active in registered state")
	}
}
