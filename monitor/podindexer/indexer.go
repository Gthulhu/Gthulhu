// SPDX-FileCopyrightText: 2025 Gthulhu Team
//
// SPDX-License-Identifier: Apache-2.0

// Package podindexer keeps a node-local pod UID -> PodRef index in
// PodMapper up to date. Without this index, PodMapper.resolvePIDtoPod
// can never associate a /proc/<pid>/cgroup-derived podUID with an
// actual Kubernetes pod, so the CRD watcher would never push any PID
// into the BPF monitored_pids map and the collector would report zero
// pod metrics.
package podindexer

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/Gthulhu/Gthulhu/monitor/collector"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

// Indexer periodically lists pods on the local node and pushes a fresh
// podUID -> PodRef map into the supplied PodMapper.
type Indexer struct {
	logger    *slog.Logger
	client    kubernetes.Interface
	podMapper *collector.PodMapper
	nodeName  string
	interval  time.Duration
}

// New creates a node-scoped pod indexer.
func New(
	kubeConfig *rest.Config,
	podMapper *collector.PodMapper,
	nodeName string,
	interval time.Duration,
	logger *slog.Logger,
) (*Indexer, error) {
	if podMapper == nil {
		return nil, fmt.Errorf("nil podMapper")
	}
	if nodeName == "" {
		return nil, fmt.Errorf("nodeName is required (set NODE_NAME env via downward API)")
	}
	if interval <= 0 {
		interval = 30 * time.Second
	}
	if logger == nil {
		logger = slog.Default()
	}
	client, err := kubernetes.NewForConfig(kubeConfig)
	if err != nil {
		return nil, fmt.Errorf("build typed client: %w", err)
	}
	return &Indexer{
		logger:    logger,
		client:    client,
		podMapper: podMapper,
		nodeName:  nodeName,
		interval:  interval,
	}, nil
}

// Run lists pods immediately and then on every tick until ctx is done.
func (i *Indexer) Run(ctx context.Context) error {
	if err := i.refresh(ctx); err != nil {
		i.logger.Warn("initial pod index refresh failed", "error", err)
	}
	ticker := time.NewTicker(i.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-ticker.C:
			if err := i.refresh(ctx); err != nil {
				i.logger.Warn("pod index refresh failed", "error", err)
			}
		}
	}
}

func (i *Indexer) refresh(ctx context.Context) error {
	selector := fields.OneTermEqualSelector("spec.nodeName", i.nodeName).String()
	list, err := i.client.CoreV1().Pods("").List(ctx, metav1.ListOptions{
		FieldSelector: selector,
	})
	if err != nil {
		return fmt.Errorf("list pods on node %s: %w", i.nodeName, err)
	}
	index := make(map[string]*collector.PodRef, len(list.Items))
	for idx := range list.Items {
		p := &list.Items[idx]
		if p.Status.Phase == corev1.PodSucceeded || p.Status.Phase == corev1.PodFailed {
			continue
		}
		uid := string(p.UID)
		if uid == "" {
			continue
		}
		index[uid] = &collector.PodRef{
			PodName:   p.Name,
			PodUID:    uid,
			Namespace: p.Namespace,
			NodeName:  p.Spec.NodeName,
			Labels:    p.Labels,
		}
	}
	i.podMapper.SetPodIndex(index)
	i.logger.Debug("pod index refreshed", "node", i.nodeName, "count", len(index))
	return nil
}
