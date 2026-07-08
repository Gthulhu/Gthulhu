package daemon

type fileRuntimeConfigStore struct{}

func (fileRuntimeConfigStore) InitializeRuntimeConfig(bootstrapConfigPath, runtimeConfigPath string) error {
	return initializeRuntimeConfig(bootstrapConfigPath, runtimeConfigPath)
}

func (fileRuntimeConfigStore) ApplyRuntimeConfig(runtimeConfigPath, schedulerBinPath string, req runtimeConfigRequest) (bool, error) {
	return applyRuntimeConfigToFile(runtimeConfigPath, schedulerBinPath, req)
}

type defaultSchedulerCommandResolver struct{}

func (defaultSchedulerCommandResolver) Resolve(configPath, gthulhuBin string) (string, []string, bool, error) {
	return schedulerCommandFromConfig(configPath, gthulhuBin)
}
