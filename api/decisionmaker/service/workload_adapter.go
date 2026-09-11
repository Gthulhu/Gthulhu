package service

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

// WorkloadAdapter maps a snapshot of node tasks into explicit, explainable
// task roles for one workload (e.g. free5GC/UPF, see #146). Adapters must
// use deterministic signals - comm, container identity, pod labels, explicit
// role registration - never ML inference or heuristic guessing.
//
// Adding a new adapter:
//  1. implement WorkloadAdapter with a stable, unique Name();
//  2. match deterministically against the given []domain.TaskIdentity (or
//     whatever explicit signal the workload exposes at match time);
//  3. return one domain.RoleMatch per matched task, with Reason explaining
//     what matched, so results can be shown on a preview/explain path
//     instead of applied silently;
//  4. register the adapter with a WorkloadAdapterRegistry;
//  5. add fixtures/tests, including a case where a non-leader thread (a
//     worker whose comm differs from its TGID leader's) is matched
//     independently of its leader - see FixtureWorkloadAdapter and
//     workload_adapter_test.go for a worked example.
type WorkloadAdapter interface {
	// Name identifies the adapter, e.g. "free5gc-upf". Used as RoleMatch.Adapter.
	Name() string
	// Match inspects tasks and returns one RoleMatch per matched task. Tasks
	// that match no role are omitted from the result, not zero-valued.
	Match(ctx context.Context, tasks []domain.TaskIdentity) ([]domain.RoleMatch, error)
}

// WorkloadAdapterRegistry holds the set of workload adapters active on a
// node. Registration is explicit and adapters are never auto-discovered,
// matching the "no generic detect every workload engine" non-goal in #146.
type WorkloadAdapterRegistry struct {
	mu       sync.RWMutex
	adapters map[string]WorkloadAdapter
}

// NewWorkloadAdapterRegistry creates an empty registry.
func NewWorkloadAdapterRegistry() *WorkloadAdapterRegistry {
	return &WorkloadAdapterRegistry{adapters: make(map[string]WorkloadAdapter)}
}

// Register adds an adapter under its Name(). It errors if the name is empty
// or already registered, so two adapters can never silently shadow each
// other.
func (r *WorkloadAdapterRegistry) Register(adapter WorkloadAdapter) error {
	if adapter == nil {
		return fmt.Errorf("workload adapter: nil adapter")
	}
	name := adapter.Name()
	if name == "" {
		return fmt.Errorf("workload adapter: empty name")
	}

	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.adapters[name]; exists {
		return fmt.Errorf("workload adapter: %q already registered", name)
	}
	r.adapters[name] = adapter
	return nil
}

// Names returns the registered adapter names in sorted order.
func (r *WorkloadAdapterRegistry) Names() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()
	names := make([]string, 0, len(r.adapters))
	for name := range r.adapters {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// MatchAll runs every registered adapter over the same task snapshot and
// returns their combined role matches, ordered by adapter name then TID for
// deterministic output. This is a preview surface: callers decide whether
// and how to act on the returned matches - MatchAll never applies anything
// to the runtime scheduler itself, matching the "expose matches through the
// preview/provenance path rather than silently applying them" requirement
// in #146.
func (r *WorkloadAdapterRegistry) MatchAll(ctx context.Context, tasks []domain.TaskIdentity) ([]domain.RoleMatch, error) {
	r.mu.RLock()
	names := make([]string, 0, len(r.adapters))
	adapters := make(map[string]WorkloadAdapter, len(r.adapters))
	for name, adapter := range r.adapters {
		names = append(names, name)
		adapters[name] = adapter
	}
	r.mu.RUnlock()
	sort.Strings(names)

	var matches []domain.RoleMatch
	for _, name := range names {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		found, err := adapters[name].Match(ctx, tasks)
		if err != nil {
			return nil, fmt.Errorf("workload adapter %q: %w", name, err)
		}
		matches = append(matches, found...)
	}
	sort.SliceStable(matches, func(i, j int) bool {
		if matches[i].Adapter != matches[j].Adapter {
			return matches[i].Adapter < matches[j].Adapter
		}
		return matches[i].Identity.TID < matches[j].Identity.TID
	})
	return matches, nil
}
