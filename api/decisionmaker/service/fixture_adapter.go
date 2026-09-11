package service

import (
	"context"

	"github.com/Gthulhu/api/decisionmaker/domain"
)

// CommRule matches a task by exact comm name and assigns it a role. It is
// the simplest deterministic signal a WorkloadAdapter can use; a real
// adapter (e.g. free5GC/UPF, #146) would typically also use container
// identity or pod labels.
type CommRule struct {
	Comm string
	Role string
}

// FixtureWorkloadAdapter is a minimal, deterministic WorkloadAdapter used as
// a reference implementation and as a test double for the adapter
// interface. A real workload adapter would match against a richer set of
// explicit signals, but the shape of Match stays the same: given a task
// snapshot, return zero or more explained role matches.
type FixtureWorkloadAdapter struct {
	name  string
	rules []CommRule
}

// NewFixtureWorkloadAdapter creates a WorkloadAdapter that assigns a role to
// any task whose comm exactly matches one of rules. The first matching rule
// wins.
func NewFixtureWorkloadAdapter(name string, rules []CommRule) *FixtureWorkloadAdapter {
	return &FixtureWorkloadAdapter{name: name, rules: rules}
}

// Name returns the adapter's registered name.
func (a *FixtureWorkloadAdapter) Name() string { return a.name }

// Match implements WorkloadAdapter.
func (a *FixtureWorkloadAdapter) Match(ctx context.Context, tasks []domain.TaskIdentity) ([]domain.RoleMatch, error) {
	var matches []domain.RoleMatch
	for _, task := range tasks {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		for _, rule := range a.rules {
			if task.Comm != rule.Comm {
				continue
			}
			matches = append(matches, domain.RoleMatch{
				Identity: task,
				Role:     rule.Role,
				Adapter:  a.name,
				Reason:   "comm == " + rule.Comm,
			})
			break
		}
	}
	return matches, nil
}
