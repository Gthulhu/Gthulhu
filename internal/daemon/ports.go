package daemon

// RuntimeConfigStore defines runtime config persistence and mutation behavior.
type RuntimeConfigStore interface {
	InitializeRuntimeConfig(bootstrapConfigPath, runtimeConfigPath string) error
	ApplyRuntimeConfig(runtimeConfigPath, schedulerBinPath string, req runtimeConfigRequest) (bool, error)
}

// SchedulerCommandResolver resolves which scheduler command daemon should run.
type SchedulerCommandResolver interface {
	Resolve(configPath, gthulhuBin string) (string, []string, bool, error)
}
