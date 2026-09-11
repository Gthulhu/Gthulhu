package service

import (
	"context"
	"errors"
	"testing"

	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestFixtureWorkloadAdapterMatchesLeaderAndNonLeaderThreadsIndependently
// exercises exactly the case a single-level /proc scan would miss (see
// #135): a UPF-style process whose thread-group leader has one comm while a
// worker thread in the same TGID has a different comm. Both must be
// matched, independently, to their own role.
func TestFixtureWorkloadAdapterMatchesLeaderAndNonLeaderThreadsIndependently(t *testing.T) {
	adapter := NewFixtureWorkloadAdapter("free5gc-upf-fixture", []CommRule{
		{Comm: "upf", Role: "upf-control"},
		{Comm: "gtp5g-worker", Role: "gtp5g-packet-worker"},
	})

	tasks := []domain.TaskIdentity{
		{TGID: 4200, TID: 4200, Starttime: 100, Comm: "upf"},          // leader
		{TGID: 4200, TID: 4201, Starttime: 100, Comm: "gtp5g-worker"}, // non-leader worker
		{TGID: 4200, TID: 4202, Starttime: 100, Comm: "idle-helper"},  // no rule matches
		{TGID: 9000, TID: 9000, Starttime: 5, Comm: "sshd"},           // unrelated process
	}

	matches, err := adapter.Match(context.Background(), tasks)
	require.NoError(t, err)
	require.Len(t, matches, 2)

	byTID := make(map[int]domain.RoleMatch, len(matches))
	for _, m := range matches {
		byTID[m.Identity.TID] = m
	}

	leader, ok := byTID[4200]
	require.True(t, ok, "leader thread must be matched")
	assert.Equal(t, "upf-control", leader.Role)
	assert.Equal(t, 4200, leader.Identity.TGID)
	assert.Equal(t, uint64(100), leader.Identity.Starttime)
	assert.Equal(t, "free5gc-upf-fixture", leader.Adapter)

	worker, ok := byTID[4201]
	require.True(t, ok, "non-leader worker thread must be matched independently of its leader")
	assert.Equal(t, "gtp5g-packet-worker", worker.Role)
	assert.Equal(t, 4200, worker.Identity.TGID, "worker keeps its TGID even though its comm differs from the leader's")
}

func TestFixtureWorkloadAdapterNoMatches(t *testing.T) {
	adapter := NewFixtureWorkloadAdapter("empty-fixture", nil)
	matches, err := adapter.Match(context.Background(), []domain.TaskIdentity{{TGID: 1, TID: 1, Comm: "init"}})
	require.NoError(t, err)
	assert.Empty(t, matches)
}

func TestFixtureWorkloadAdapterContextCancelled(t *testing.T) {
	adapter := NewFixtureWorkloadAdapter("fixture", []CommRule{{Comm: "upf", Role: "upf-control"}})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := adapter.Match(ctx, []domain.TaskIdentity{{TGID: 1, TID: 1, Comm: "upf"}})
	require.ErrorIs(t, err, context.Canceled)
}

func TestWorkloadAdapterRegistryRegisterRejectsNilAndEmptyName(t *testing.T) {
	reg := NewWorkloadAdapterRegistry()
	require.Error(t, reg.Register(nil))
	require.Error(t, reg.Register(NewFixtureWorkloadAdapter("", nil)))
	assert.Empty(t, reg.Names())
}

func TestWorkloadAdapterRegistryRegisterRejectsDuplicateName(t *testing.T) {
	reg := NewWorkloadAdapterRegistry()
	require.NoError(t, reg.Register(NewFixtureWorkloadAdapter("dup", nil)))
	err := reg.Register(NewFixtureWorkloadAdapter("dup", nil))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "already registered")
	assert.Equal(t, []string{"dup"}, reg.Names())
}

func TestWorkloadAdapterRegistryMatchAllAggregatesAndOrdersDeterministically(t *testing.T) {
	reg := NewWorkloadAdapterRegistry()
	require.NoError(t, reg.Register(NewFixtureWorkloadAdapter("z-adapter", []CommRule{{Comm: "upf", Role: "upf-control"}})))
	require.NoError(t, reg.Register(NewFixtureWorkloadAdapter("a-adapter", []CommRule{{Comm: "gtp5g-worker", Role: "gtp5g-packet-worker"}})))

	tasks := []domain.TaskIdentity{
		{TGID: 1, TID: 2, Comm: "gtp5g-worker"},
		{TGID: 1, TID: 1, Comm: "upf"},
	}

	matches, err := reg.MatchAll(context.Background(), tasks)
	require.NoError(t, err)
	require.Len(t, matches, 2)
	// "a-adapter" sorts before "z-adapter" regardless of registration order.
	assert.Equal(t, "a-adapter", matches[0].Adapter)
	assert.Equal(t, "z-adapter", matches[1].Adapter)
}

func TestWorkloadAdapterRegistryMatchAllPropagatesAdapterError(t *testing.T) {
	reg := NewWorkloadAdapterRegistry()
	require.NoError(t, reg.Register(&erroringAdapter{name: "broken"}))
	_, err := reg.MatchAll(context.Background(), []domain.TaskIdentity{{TGID: 1, TID: 1, Comm: "x"}})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "broken")
}

func TestWorkloadAdapterRegistryMatchAllContextCancelled(t *testing.T) {
	reg := NewWorkloadAdapterRegistry()
	require.NoError(t, reg.Register(NewFixtureWorkloadAdapter("fixture", nil)))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := reg.MatchAll(ctx, []domain.TaskIdentity{{TGID: 1, TID: 1, Comm: "x"}})
	require.ErrorIs(t, err, context.Canceled)
}

// erroringAdapter is a WorkloadAdapter test double that always fails, used
// to verify MatchAll surfaces (rather than swallows) an adapter error.
type erroringAdapter struct{ name string }

func (a *erroringAdapter) Name() string { return a.name }
func (a *erroringAdapter) Match(ctx context.Context, tasks []domain.TaskIdentity) ([]domain.RoleMatch, error) {
	return nil, errors.New("boom")
}
