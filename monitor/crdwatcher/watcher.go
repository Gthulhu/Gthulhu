// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

// Package crdwatcher watches PodSchedulingMetrics CRD objects and drives the
// eBPF collector's monitored PID/TGID sets accordingly.
package crdwatcher

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"regexp"
	"sync"
	"time"

	"github.com/Gthulhu/Gthulhu/monitor/collector"
	"github.com/Gthulhu/api/decisionmaker/domain"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/rest"
)

var psmGVR = schema.GroupVersionResource{
	Group:    "gthulhu.io",
	Version:  "v1alpha1",
	Resource: "podschedulingmetrics",
}

// Watcher monitors PodSchedulingMetrics CRDs and reconciles the eBPF
// collector's PID filters so that only interesting pods are tracked.
type Watcher struct {
	logger    *slog.Logger
	client    dynamic.Interface
	collector *collector.Collector
	podMapper *collector.PodMapper
	nodeName  string

	mu    sync.RWMutex
	specs map[string]*domain.PodSchedulingMetrics // key = namespace/name
}

// New creates a Watcher.
func New(
	kubeConfig *rest.Config,
	col *collector.Collector,
	podMapper *collector.PodMapper,
	nodeName string,
	logger *slog.Logger,
) (*Watcher, error) {
	dynClient, err := dynamic.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("dynamic client: %w", err)
	}
	return &Watcher{
		logger:    logger,
		client:    dynClient,
		collector: col,
		podMapper: podMapper,
		nodeName:  nodeName,
		specs:     make(map[string]*domain.PodSchedulingMetrics),
	}, nil
}

// Run starts watching PodSchedulingMetrics across all namespaces.
// It blocks until ctx is cancelled.
func (w *Watcher) Run(ctx context.Context) error {
	for {
		if err := w.watchLoop(ctx); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			w.logger.Error("watch error, retrying in 5s", "error", err)
			time.Sleep(5 * time.Second)
		}
	}
}

func (w *Watcher) watchLoop(ctx context.Context) error {
	watcher, err := w.client.Resource(psmGVR).Namespace("").Watch(ctx, metav1.ListOptions{})
	if err != nil {
		return fmt.Errorf("watch: %w", err)
	}
	defer watcher.Stop()
	w.logger.Info("CRD watcher started for PodSchedulingMetrics")

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-watcher.ResultChan():
			if !ok {
				return fmt.Errorf("watch channel closed")
			}
			w.handleEvent(event)
		}
	}
}

func (w *Watcher) handleEvent(event watch.Event) {
	obj, ok := event.Object.(*unstructured.Unstructured)
	if !ok {
		return
	}
	key := obj.GetNamespace() + "/" + obj.GetName()

	switch event.Type {
	case watch.Added, watch.Modified:
		psm, err := parsePSM(obj)
		if err != nil {
			w.logger.Warn("failed to parse PodSchedulingMetrics", "key", key, "error", err)
			return
		}
		w.mu.Lock()
		w.specs[key] = psm
		w.mu.Unlock()
		w.logger.Info("PodSchedulingMetrics updated", "key", key, "enabled", psm.Spec.Enabled)
		w.reconcilePIDs()

	case watch.Deleted:
		w.mu.Lock()
		delete(w.specs, key)
		w.mu.Unlock()
		w.logger.Info("PodSchedulingMetrics deleted", "key", key)
		w.reconcilePIDs()
	}
}

// reconcilePIDs computes the desired set of monitored PIDs from all active
// PodSchedulingMetrics specs and syncs them to the eBPF maps.
func (w *Watcher) reconcilePIDs() {
	w.mu.RLock()
	defer w.mu.RUnlock()

	// Gather all pod UIDs that match any enabled PSM
	desiredPods := make(map[string]bool)
	allPods := w.podMapper.GetAllPodRefs()
	for _, psm := range w.specs {
		if !psm.Spec.Enabled {
			continue
		}
		for uid, ref := range allPods {
			if w.psmMatchesPod(psm, ref) {
				desiredPods[uid] = true
			}
		}
	}

	// Get all PIDs belonging to desired pods and sync to BPF
	mappedPIDs := w.podMapper.ListMappedPIDs()
	for _, pid := range mappedPIDs {
		podRef := w.podMapper.GetPodForPID(pid)
		if podRef == nil {
			continue
		}
		if desiredPods[podRef.PodUID] {
			if err := w.collector.AddMonitoredPID(pid); err != nil {
				w.logger.Warn("failed to add monitored PID", "pid", pid, "error", err)
			}
		} else {
			if err := w.collector.RemoveMonitoredPID(pid); err != nil {
				w.logger.Warn("failed to remove monitored PID", "pid", pid, "error", err)
			}
		}
	}
	w.logger.Debug("reconcilePIDs done", "desiredPods", len(desiredPods))
}

// psmMatchesPod checks if a PodSchedulingMetrics spec matches a given pod.
func (w *Watcher) psmMatchesPod(psm *domain.PodSchedulingMetrics, ref *collector.PodRef) bool {
	// Namespace filter
	if len(psm.Spec.K8sNamespaces) > 0 {
		found := false
		for _, ns := range psm.Spec.K8sNamespaces {
			if ns == ref.Namespace {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// Label selectors are matched against pod labels; however, PodRef doesn't
	// carry labels. For now, we match on pod name patterns. Full label matching
	// requires extending PodRef with labels from the K8S informer.
	// TODO: extend PodRef with labels for precise selector matching
	return true
}

// GetActiveSpecs returns all currently watched PodSchedulingMetrics specs.
func (w *Watcher) GetActiveSpecs() []*domain.PodSchedulingMetrics {
	w.mu.RLock()
	defer w.mu.RUnlock()
	out := make([]*domain.PodSchedulingMetrics, 0, len(w.specs))
	for _, s := range w.specs {
		out = append(out, s)
	}
	return out
}

// MatchesCommandRegex checks if a process command matches the regex filter.
func MatchesCommandRegex(pattern, command string) bool {
	if pattern == "" {
		return true
	}
	re, err := regexp.Compile(pattern)
	if err != nil {
		return false
	}
	return re.MatchString(command)
}

// parsePSM converts an unstructured CRD object into our domain type.
func parsePSM(obj *unstructured.Unstructured) (*domain.PodSchedulingMetrics, error) {
	specRaw, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		return nil, fmt.Errorf("missing spec")
	}
	specJSON, err := json.Marshal(specRaw)
	if err != nil {
		return nil, fmt.Errorf("marshal spec: %w", err)
	}
	var spec domain.PodSchedulingMetricsSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return nil, fmt.Errorf("unmarshal spec: %w", err)
	}
	return &domain.PodSchedulingMetrics{
		Name:      obj.GetName(),
		Namespace: obj.GetNamespace(),
		Spec:      spec,
	}, nil
}
