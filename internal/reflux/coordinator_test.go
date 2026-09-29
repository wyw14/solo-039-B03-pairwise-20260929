package reflux

import "testing"

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

	coordinator.Release(lease)
	if used := coordinator.Used(); used != 0 {
		t.Fatalf("used after release = %f, want 0", used)
	}

	// Repeated releases of the same lease must return no extra capacity.
	coordinator.Release(lease)
	coordinator.Release(lease)
	if used := coordinator.Used(); used != 0 {
		t.Fatalf("used after duplicate release = %f, want 0", used)
	}

	// Capacity must be fully available again for a new lease.
	if _, err := coordinator.Acquire("T-102", 1, 1); err != nil {
		t.Fatalf("re-acquire after idempotent release failed: %v", err)
	}
}

func TestCoordinatorDuplicateReleaseCannotInflateCapacity(t *testing.T) {
	coordinator := NewCoordinator(1)
	lease, err := coordinator.Acquire("T-101", 0.6, 1)
	if err != nil {
		t.Fatal(err)
	}

	coordinator.Release(lease)
	coordinator.Release(lease)

	// Even if a caller retries the release, a full-capacity lease must still
	// be rejected while capacity is occupied.
	if _, err := coordinator.Acquire("T-102", 0.6, 1); err != nil {
		t.Fatal(err)
	}
	if _, err := coordinator.Acquire("T-103", 0.6, 1); err == nil {
		t.Fatal("lease exceeding pump capacity granted after duplicate release")
	}
	if used := coordinator.Used(); used > 1 {
		t.Fatalf("pump capacity exceeded: used=%f", used)
	}
}

func TestCoordinatorStaleFenceAcquireRejected(t *testing.T) {
	coordinator := NewCoordinator(1)
	newGen, err := coordinator.Acquire("T-101", 0.6, 2)
	if err != nil {
		t.Fatal(err)
	}

	// A retried request from the previous generation cannot take capacity.
	if _, err := coordinator.Acquire("T-101", 0.6, 1); err == nil {
		t.Fatal("stale-fence acquire was accepted")
	}
	if used := coordinator.Used(); used != 0.6 {
		t.Fatalf("used = %f, want 0.6", used)
	}

	// Releasing with a stale fence copy is a no-op; the live lease keeps its
	// capacity until released with its own generation.
	stale := newGen
	stale.Fence = 1
	coordinator.Release(stale)
	if used := coordinator.Used(); used != 0.6 {
		t.Fatalf("stale-fence release changed used: %f, want 0.6", used)
	}

	coordinator.Release(newGen)
	if used := coordinator.Used(); used != 0 {
		t.Fatalf("used after valid release = %f, want 0", used)
	}
}

func TestCoordinatorDistinctLeasesReleasedSeparately(t *testing.T) {
	coordinator := NewCoordinator(1)
	first, err := coordinator.Acquire("T-101", 0.4, 1)
	if err != nil {
		t.Fatal(err)
	}
	second, err := coordinator.Acquire("T-101", 0.4, 1)
	if err != nil {
		t.Fatal(err)
	}

	coordinator.Release(first)
	coordinator.Release(first) // duplicate of the first
	if used := coordinator.Used(); used != 0.4 {
		t.Fatalf("used = %f, want 0.4 (second lease still active)", used)
	}

	coordinator.Release(second)
	if used := coordinator.Used(); used != 0 {
		t.Fatalf("used = %f, want 0", used)
	}
}
