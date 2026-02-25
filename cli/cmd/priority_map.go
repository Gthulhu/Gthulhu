// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

var (
	priorityMapNode      string
	priorityMapLabel     string
	priorityMapContainer string
	priorityMapNames     []string
	priorityMapLocal     bool
)

var priorityMapCmd = &cobra.Command{
	Use:   "priority-map",
	Short: "View the BPF priority map from gthulhu scheduler pods",
	Long: `Inspect the priority_tasks BPF map on scheduler nodes.

In local mode (--local), the command runs bpftool directly on the current
host (requires root privileges and bpftool installed).

In Kubernetes mode (default), the command uses kubectl exec to run bpftool
inside scheduler pods. Use --label to customize the pod selector and --node
to target a specific node.

By default, it dumps both priority_tasks and priority_tasks_prio maps. Use
--map-name to override the list of maps to dump.

Examples:
  # View priority map on all scheduler pods
  gthulhu-cli priority-map

  # View priority map on a specific node
  gthulhu-cli priority-map --node worker-1

  # View priority map locally (on the current host)
  gthulhu-cli priority-map --local`,
	RunE: runPriorityMap,
}

func init() {
	priorityMapCmd.Flags().StringVar(&priorityMapNode, "node", "", "Target a specific Kubernetes node")
	priorityMapCmd.Flags().StringVar(&priorityMapLabel, "label", "app.kubernetes.io/component=scheduler,app.kubernetes.io/instance=gthulhu,app.kubernetes.io/name=gthulhu", "Label selector for scheduler pods")
	priorityMapCmd.Flags().StringVar(&priorityMapContainer, "container", "scheduler-sidecar", "Container name to exec into for bpftool")
	priorityMapCmd.Flags().StringSliceVar(&priorityMapNames, "map-name", []string{"priority_tasks", "priority_tasks_prio"}, "BPF map names to dump (repeatable)")
	priorityMapCmd.Flags().BoolVar(&priorityMapLocal, "local", false, "Read the BPF map on the local host (requires root)")
	rootCmd.AddCommand(priorityMapCmd)
}

func runPriorityMap(cmd *cobra.Command, args []string) error {
	priorityMapNames = normalizeMapNames(priorityMapNames)
	if priorityMapLocal {
		return runLocalPriorityMap()
	}
	return runK8sPriorityMap()
}

// runLocalPriorityMap dumps the requested BPF maps on the local host.
func runLocalPriorityMap() error {
	for _, mapName := range priorityMapNames {
		fmt.Printf("=== Map: %s ===\n", mapName)
		out, err := execCommand("bpftool", "map", "dump", "name", mapName)
		if err != nil {
			return fmt.Errorf("bpftool (%s): %w\n%s", mapName, err, out)
		}
		fmt.Println(out)
	}
	return nil
}

// runK8sPriorityMap discovers scheduler pods via kubectl and dumps the
// priority_tasks BPF map from each one.
func runK8sPriorityMap() error {
	kubecfgArgs := kubectlConfigArgs()

	// List scheduler pods (optionally filtered by node).
	listArgs := append(kubecfgArgs,
		"get", "pods",
		"-l", priorityMapLabel,
		"-o", "jsonpath={range .items[*]}{.metadata.name} {.spec.nodeName}{'\\n'}{end}",
	)
	out, err := execCommand("kubectl", listArgs...)
	if err != nil {
		return fmt.Errorf("kubectl get pods: %w\n%s", err, out)
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return fmt.Errorf("no scheduler pods found (label=%s, namespace=%s)", priorityMapLabel, namespace)
	}

	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		podName, nodeName := parts[0], parts[1]

		// Skip pods not on the requested node (if --node is set).
		if priorityMapNode != "" && nodeName != priorityMapNode {
			continue
		}

		fmt.Printf("=== Node: %s | Pod: %s ===\n", nodeName, podName)

		containerName, err := resolveBpftoolContainer(podName, kubecfgArgs)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n", err)
			continue
		}

		for _, mapName := range priorityMapNames {
			execArgs := append(kubecfgArgs, "exec", podName, "-c", containerName,
				"--",
				"bpftool", "map", "dump", "name", mapName,
			)
			mapOut, err := execCommand("kubectl", execArgs...)
			if err != nil {
				fmt.Fprintf(os.Stderr, "  error (%s): %v\n%s\n", mapName, err, mapOut)
				continue
			}
			fmt.Printf("--- Map: %s ---\n", mapName)
			fmt.Println(mapOut)
		}
	}
	return nil
}

// kubectlConfigArgs returns common kubectl flags derived from CLI flags.
func kubectlConfigArgs() []string {
	var args []string
	if kubeconfig != "" {
		args = append(args, "--kubeconfig", kubeconfig)
	}
	if namespace != "" {
		args = append(args, "-n", namespace)
	}
	return args
}

func resolveBpftoolContainer(podName string, kubecfgArgs []string) (string, error) {
	listArgs := append(kubecfgArgs,
		"get", "pod", podName,
		"-o", "jsonpath={.spec.containers[*].name}",
	)
	containersOut, err := execCommand("kubectl", listArgs...)
	if err != nil {
		return "", fmt.Errorf("kubectl get pod containers: %w\n%s", err, containersOut)
	}
	containers := strings.Fields(strings.TrimSpace(containersOut))
	if len(containers) == 0 {
		return "", fmt.Errorf("no containers found in pod %s", podName)
	}

	tryContainers := []string{}
	if priorityMapContainer != "" {
		tryContainers = append(tryContainers, priorityMapContainer)
	}
	for _, c := range containers {
		if c != priorityMapContainer {
			tryContainers = append(tryContainers, c)
		}
	}

	for _, container := range tryContainers {
		checkArgs := append(kubecfgArgs, "exec", podName, "-c", container,
			"--", "sh", "-c", "command -v bpftool",
		)
		if _, err := execCommand("kubectl", checkArgs...); err == nil {
			return container, nil
		}
	}

	return "", fmt.Errorf("bpftool not found in any container for pod %s (tried: %s)", podName, strings.Join(tryContainers, ", "))
}

func normalizeMapNames(mapNames []string) []string {
	if len(mapNames) == 0 {
		return mapNames
	}
	seen := map[string]bool{}
	result := make([]string, 0, len(mapNames))
	for _, name := range mapNames {
		mapped := name
		if name == "priority_tasks_prio" {
			mapped = "priority_tasks_"
		}
		if !seen[mapped] {
			seen[mapped] = true
			result = append(result, mapped)
		}
	}
	return result
}

// execCommand runs an external command and returns its combined output.
func execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}
