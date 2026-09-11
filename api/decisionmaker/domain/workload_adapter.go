package domain

// TaskIdentity is the portable identity of one kernel scheduling entity - a
// thread, not a process - that a workload adapter reasons about. TGID ties a
// thread to its group, TID is the thread the Linux scheduler actually runs,
// and Starttime (jiffies since boot, /proc/<tgid>/task/<tid>/stat field 22)
// disambiguates a task from any future task that reuses the same TID after
// exit, so a stale role match can never be mistaken for a live one.
type TaskIdentity struct {
	TGID      int
	TID       int
	Starttime uint64
	Comm      string
}

// RoleMatch is one task-role assignment produced by a WorkloadAdapter. It
// carries enough identity and provenance to be shown on a preview/explain
// path before anything is applied to the runtime scheduler, matching the
// Claim2Core principle (#141) that matches must be provable, not silently
// applied.
type RoleMatch struct {
	Identity TaskIdentity
	Role     string
	Adapter  string
	Reason   string
}
