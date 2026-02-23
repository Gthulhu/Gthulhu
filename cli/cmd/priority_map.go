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
	priorityMapNode  string
	priorityMapLabel string
	priorityMapLocal bool
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
	priorityMapCmd.Flags().StringVar(&priorityMapLabel, "label", "app=gthulhu", "Label selector for scheduler pods")
	priorityMapCmd.Flags().BoolVar(&priorityMapLocal, "local", false, "Read the BPF map on the local host (requires root)")
	rootCmd.AddCommand(priorityMapCmd)
}

func runPriorityMap(cmd *cobra.Command, args []string) error {
	if priorityMapLocal {
		return runLocalPriorityMap()
	}
	return runK8sPriorityMap()
}

// runLocalPriorityMap dumps the priority_tasks BPF map on the local host.
func runLocalPriorityMap() error {
	out, err := execCommand("bpftool", "map", "dump", "name", "priority_tasks")
	if err != nil {
		return fmt.Errorf("bpftool: %w\n%s", err, out)
	}
	fmt.Println(out)
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

		execArgs := append(kubecfgArgs,
			"exec", podName, "--",
			"bpftool", "map", "dump", "name", "priority_tasks",
		)
		mapOut, err := execCommand("kubectl", execArgs...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "  error: %v\n%s\n", err, mapOut)
			continue
		}
		fmt.Println(mapOut)
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

// execCommand runs an external command and returns its combined output.
func execCommand(name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	var buf bytes.Buffer
	cmd.Stdout = &buf
	cmd.Stderr = &buf
	err := cmd.Run()
	return buf.String(), err
}
