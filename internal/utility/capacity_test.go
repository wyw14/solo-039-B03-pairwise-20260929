package utility

import (
	"testing"

	"github.com/google/uuid"
	"github.com/wyw14/solo-039/internal/model"
)

func TestCapacityReleaseIsIdempotent(t *testing.T) {
	capacity := NewCapacity(1)
	lease, err := capacity.Reserve("T-101", 0.6, 1)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := capacity.Reserve("T-102", 0.4, 1); err != nil {
		t.Fatal(err)
	}
	capacity.Release(lease)
	capacity.Release(lease)
	if got := capacity.Used(); got != 0.4 {
		t.Fatalf("duplicate release freed capacity twice: used=%f", got)
	}
	if _, err := capacity.Reserve("T-103", 0.7, 1); err == nil {
		t.Fatal("duplicate release manufactured capacity beyond the limit")
	}
}

func TestCapacityIgnoresStaleAndUnknownLeases(t *testing.T) {
	capacity := NewCapacity(1)
	first, err := capacity.Reserve("T-101", 0.5, 1)
	if err != nil {
		t.Fatal(err)
	}
	capacity.Release(first)
	second, err := capacity.Reserve("T-101", 0.5, 2)
	if err != nil {
		t.Fatal(err)
	}
	capacity.Release(first)
	forged := second
	forged.Fence = 1
	capacity.Release(forged)
	capacity.Release(model.Lease{ID: uuid.New(), Owner: "T-109", Capacity: 0.9, Fence: 1, Active: true})
	capacity.Release(model.Lease{})
	if got := capacity.Used(); got != 0.5 {
		t.Fatalf("stale or unknown release freed live capacity: used=%f", got)
	}
	capacity.Release(second)
	if got := capacity.Used(); got != 0 {
		t.Fatalf("valid lease was not returned exactly once: used=%f", got)
	}
}

func TestCapacityReserveAssignsUniqueLeaseIDs(t *testing.T) {
	capacity := NewCapacity(1)
	first, err := capacity.Reserve("T-101", 0.4, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := capacity.Reserve("T-102", 0.4, 1)
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == second.ID {
		t.Fatal("reserved leases share an ID; releases cannot be matched idempotently")
	}
}
