package util

import "os"

func GetMachineID() string {
	machineID := os.Getenv("MACHINE_ID")
	if machineID != "" {
		return machineID
	}
	// On Linux, the machine ID is stored in /etc/machine-id
	const machineIDPath = "/etc/machine-id"
	data, err := os.ReadFile(machineIDPath)
	if err != nil {
		return "unknown-machine-id"
	}
	return string(data)
}
