package service

import (
	"bufio"
	"context"
	"crypto/rsa"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Gthulhu/api/config"
	"github.com/Gthulhu/api/decisionmaker/domain"
	"github.com/Gthulhu/api/pkg/logger"
	"github.com/Gthulhu/api/pkg/util"
	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/fx"
)

type Params struct {
	fx.In
	TokenConfig config.TokenConfig
}

func NewService(params Params) (*Service, error) {
	privateKey, err := util.InitRSAPrivateKey(string(params.TokenConfig.RsaPrivateKeyPem))
	if err != nil {
		return nil, fmt.Errorf("failed to initialize JWT private key: %v", err)
	}
	svc := &Service{
		schedulingIntentsMap: util.NewGenericMap[string, []*domain.SchedulingIntents](),
		metricCollector:      NewMetricCollector(util.GetMachineID()),
		jwtPrivateKey:        privateKey,
	}

	err = prometheus.Register(svc.metricCollector)
	if err != nil {
		return nil, fmt.Errorf("failed to register metric collector: %v", err)
	}
	return svc, nil
}

type Service struct {
	schedulingIntentsMap *util.GenericMap[string, []*domain.SchedulingIntents]
	metricCollector      *MetricCollector
	jwtPrivateKey        *rsa.PrivateKey
	tokenConfig          config.TokenConfig
	intentCacheMu        sync.RWMutex
	intentCache          []*domain.Intent
	intentMerkleRoot     *util.MerkleNode
	intentMerkleRootHash string
}

const (
	procDir      = "/proc"
	pauseCommand = "pause"
)

// ListAllSchedulingIntents re-scans /proc and recalculates scheduling intents
// from the cached domain.Intent list, since pod processes may change over time.
func (svc *Service) ListAllSchedulingIntents(ctx context.Context) ([]*domain.SchedulingIntents, error) {
	svc.intentCacheMu.RLock()
	cachedIntents := svc.intentCache
	svc.intentCacheMu.RUnlock()

	if len(cachedIntents) == 0 {
		return []*domain.SchedulingIntents{}, nil
	}

	podInfos, err := svc.GetAllPodInfos(ctx)
	if err != nil {
		return nil, err
	}

	return svc.resolveSchedulingIntents(ctx, cachedIntents, podInfos), nil
}

// ProcessIntents processes a list of scheduling intents and updates the internal map
func (svc *Service) ProcessIntents(ctx context.Context, intents []*domain.Intent) error {
	podInfos, err := svc.GetAllPodInfos(ctx)
	if err != nil {
		return err
	}

	// update intent cache and merkle tree
	normalizedIntents := normalizeIntentInputs(intents)
	sortedIntents := sortIntentsByKey(normalizedIntents)
	leafHashes := make([]string, 0, len(sortedIntents))
	for _, intent := range sortedIntents {
		leafHashes = append(leafHashes, hashIntent(intent))
	}
	root := util.BuildMerkleTree(leafHashes)
	svc.intentCacheMu.Lock()
	svc.intentCache = normalizedIntents
	svc.intentMerkleRoot = root
	if root != nil {
		svc.intentMerkleRootHash = root.Hash
	} else {
		svc.intentMerkleRootHash = ""
	}
	svc.intentCacheMu.Unlock()
	svc.resolveSchedulingIntents(ctx, normalizedIntents, podInfos)
	logger.Logger(ctx).Info().Msgf("Discovered pods: %+v", podInfos)
	return nil
}

// resolveSchedulingIntents converts domain.Intents + PodInfos into SchedulingIntents,
// updates the schedulingIntentsMap and returns all resolved scheduling intents.
func (svc *Service) resolveSchedulingIntents(ctx context.Context, intents []*domain.Intent, podInfos map[string]*domain.PodInfo) []*domain.SchedulingIntents {
	svc.schedulingIntentsMap.Clear()
	var allSchedulingIntents []*domain.SchedulingIntents
	for _, intent := range intents {
		podInfo := podInfos[intent.PodID]
		logger.Logger(ctx).Info().Msgf("Processing intent for PodName:%s PodID: %s on NodeID: %s, Process:%+v", intent.PodName, intent.PodID, intent.NodeID, podInfo)
		labels := []domain.LabelSelector{}
		for key, value := range intent.PodLabels {
			labels = append(labels, domain.LabelSelector{
				Key:   key,
				Value: value,
			})
		}
		if podInfo != nil && len(podInfo.Processes) > 0 {
			for _, process := range podInfo.Processes {
				if process.Command == pauseCommand {
					continue
				}
				if !regexp.MustCompile(intent.CommandRegex).MatchString(process.Command) {
					continue
				}
				schedulingIntent := &domain.SchedulingIntents{
					Priority:      intent.Priority,
					ExecutionTime: uint64(intent.ExecutionTime),
					PID:           process.PID,
					CommandRegex:  intent.CommandRegex,
					Selectors:     labels,
				}
				logger.Logger(ctx).Info().Msgf("Created SchedulingIntent: %+v for Process PID: %d", schedulingIntent, process.PID)
				svc.schedulingIntentsMap.Store(fmt.Sprintf("%s-%d", intent.PodID, process.PID), []*domain.SchedulingIntents{schedulingIntent})
				allSchedulingIntents = append(allSchedulingIntents, schedulingIntent)
			}
		}
	}
	return allSchedulingIntents
}

// GetAllPodInfos retrieves all pod information by scanning the /proc filesystem
func (svc *Service) GetAllPodInfos(ctx context.Context) (map[string]*domain.PodInfo, error) {
	return svc.FindPodInfoFrom(ctx, procDir)
}

// FindPodInfoFrom scans the given rootDir (e.g., /proc) to find pod information
func (svc *Service) FindPodInfoFrom(ctx context.Context, rootDir string) (map[string]*domain.PodInfo, error) {
	podMap := make(map[string]*domain.PodInfo)

	// Walk through /proc to find all processes
	entries, err := os.ReadDir(rootDir)
	if err != nil {
		return nil, fmt.Errorf("failed to read /proc directory: %v", err)
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		// Check if directory name is a PID (numeric)
		pid, err := strconv.Atoi(entry.Name())
		if err != nil {
			// Not a numeric PID directory (e.g., "acpi", "bus", etc.) — skip
			continue
		}

		// Read cgroup information for this process
		cgroupPath := fmt.Sprintf("%s/%d/cgroup", rootDir, pid)
		file, err := os.Open(cgroupPath)
		if err != nil {
			logger.Logger(ctx).Warn().Err(err).Msgf("failed to open cgroup file for pid %d", pid)
			continue
		}
		scanner := bufio.NewScanner(file)
		for scanner.Scan() {
			line := scanner.Text()
			logger.Logger(ctx).Debug().Msgf("cgroup line for pid %d: %s", pid, line)
			if strings.Contains(line, "kubepods") {
				err = svc.parseCgroupToPodInfo(rootDir, line, pid, podMap)
				if err != nil {
					logger.Logger(ctx).Warn().Err(err).Msgf("failed to parse cgroup line for pid %d, line:%s", pid, line)
					break
				}
			}
		}
		if err := scanner.Err(); err != nil {
		}
		_ = file.Close()
	}

	return podMap, nil
}

// parseCgroupToPodInfo parses a cgroup line (e.g // 0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-pod20da609e_6973_4463_a1f9_2db9bcc5becc.slice/cri-containerd-10ec3c89629f71226b227e6510b2d465168b24005bbdcc5d7940517080830635.scope) to extract pod info and updates the podInfoMap
func (svc *Service) parseCgroupToPodInfo(rootDir string, line string, pid int, podInfoMap map[string]*domain.PodInfo) error {
	parts := strings.Split(line, ":")
	if len(parts) >= 3 {
		cgroupHierarchy := parts[2]

		// Extract pod information
		podUID, containerID, err := svc.getPodInfoFromCgroup(cgroupHierarchy)
		if err != nil {
			return err
		}

		// Get process information
		process, err := svc.getProcessInfo(rootDir, pid)
		if err != nil {
			return err
		}
		process.ContainerID = containerID

		// Create or update pod info
		if podInfo, exists := podInfoMap[podUID]; exists {
			podInfo.Processes = append(podInfo.Processes, process)
		} else {
			podInfoMap[podUID] = &domain.PodInfo{
				PodUID:    podUID,
				Processes: []domain.PodProcess{process},
			}
		}
	}
	return nil
}

// Support multiple cgroup formats:
// - systemd: kubelet-kubepods-pod20da609e_6973_4463_a1f9_2db9bcc5becc.slice (underscores)
// - cgroupfs: /kubepods/burstable/pod31e4e721-a5a0-421a-ae1d-b7971ae30d6e/ (dashes)
var podRegex = regexp.MustCompile(`pod([0-9a-fA-F]{8}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{4}[-_][0-9a-fA-F]{12})`)

// getPodInfoFromCgroup extracts pod information from cgroup path
func (svc *Service) getPodInfoFromCgroup(cgroupPath string) (podUID string, containerID string, err error) {
	// Parse cgroup path to extract pod information
	// 0::/kubelet.slice/kubelet-kubepods.slice/kubelet-kubepods-pod20da609e_6973_4463_a1f9_2db9bcc5becc.slice/cri-containerd-10ec3c89629f71226b227e6510b2d465168b24005bbdcc5d7940517080830635.scope
	parts := strings.Split(cgroupPath, "/")
	for _, part := range parts {
		if podRegex.MatchString(part) {
			podUID = podRegex.FindStringSubmatch(part)[1]
			podUID = strings.ReplaceAll(podUID, "_", "-")
		}
		if strings.HasPrefix(part, "cri-containerd-") && strings.HasSuffix(part, ".scope") {
			containerID = strings.TrimPrefix(part, "cri-containerd-")
			containerID = strings.TrimSuffix(containerID, ".scope")
		}
	}

	if podUID == "" {
		return "", "", fmt.Errorf("pod UID not found in cgroup path")
	}

	return podUID, containerID, nil
}

// getProcessInfo reads process information from /proc/<pid>/
func (svc *Service) getProcessInfo(rootDir string, pid int) (domain.PodProcess, error) {
	process := domain.PodProcess{PID: pid}

	// Read command from /proc/<pid>/comm
	commPath := fmt.Sprintf("/%s/%d/comm", rootDir, pid)
	if data, err := os.ReadFile(commPath); err == nil {
		process.Command = strings.TrimSpace(string(data))
	}

	// Read PPID from /proc/<pid>/stat
	statPath := fmt.Sprintf("/%s/%d/stat", rootDir, pid)
	if data, err := os.ReadFile(statPath); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) >= 4 {
			if ppid, err := strconv.Atoi(fields[3]); err == nil {
				process.PPID = ppid
			}
		}
	}

	return process, nil
}

func (svc *Service) UpdateMetrics(ctx context.Context, newMetricSet *domain.MetricSet) {
	svc.metricCollector.UpdateMetrics(newMetricSet)
}

func normalizeIntentInputs(intents []*domain.Intent) []*domain.Intent {
	results := make([]*domain.Intent, 0, len(intents))
	for _, intent := range intents {
		if intent == nil {
			continue
		}
		results = append(results, intent)
	}
	return results
}

func sortIntentsByKey(intents []*domain.Intent) []*domain.Intent {
	results := make([]*domain.Intent, 0, len(intents))
	results = append(results, intents...)
	sort.Slice(results, func(i, j int) bool {
		return intentSortKey(results[i]) < intentSortKey(results[j])
	})
	return results
}

func hashIntent(intent *domain.Intent) string {
	labels := make([]string, 0, len(intent.PodLabels))
	for key, value := range intent.PodLabels {
		labels = append(labels, key+"="+value)
	}
	sort.Strings(labels)
	serialized := strings.Join([]string{
		"podName=" + intent.PodName,
		"podID=" + intent.PodID,
		"nodeID=" + intent.NodeID,
		"k8sNamespace=" + intent.K8sNamespace,
		"commandRegex=" + intent.CommandRegex,
		"priority=" + strconv.Itoa(intent.Priority),
		"executionTime=" + strconv.FormatInt(intent.ExecutionTime, 10),
		"podLabels=" + strings.Join(labels, ","),
	}, "|")
	return util.HashStringSHA256Hex(serialized)
}

func intentSortKey(intent *domain.Intent) string {
	labels := make([]string, 0, len(intent.PodLabels))
	for key, value := range intent.PodLabels {
		labels = append(labels, key+"="+value)
	}
	sort.Strings(labels)
	return strings.Join([]string{
		intent.PodName,
		intent.PodID,
		intent.NodeID,
		intent.K8sNamespace,
		intent.CommandRegex,
		strconv.Itoa(intent.Priority),
		strconv.FormatInt(intent.ExecutionTime, 10),
		strings.Join(labels, ","),
	}, "|")
}

func (svc *Service) refreshIntentMerkleTreeIfNeeded() {
	var hasRoot bool

	svc.intentCacheMu.RLock()
	{
		hasRoot = svc.intentMerkleRoot != nil
	}
	svc.intentCacheMu.RUnlock()

	if hasRoot {
		return
	}

	svc.intentCacheMu.Lock()
	defer svc.intentCacheMu.Unlock()
	if svc.intentMerkleRoot != nil {
		return
	}
	normalized := normalizeIntentInputs(svc.intentCache)
	sorted := sortIntentsByKey(normalized)
	leafHashes := make([]string, 0, len(sorted))
	for _, intent := range sorted {
		leafHashes = append(leafHashes, hashIntent(intent))
	}
	root := util.BuildMerkleTree(leafHashes)
	svc.intentMerkleRoot = root
	if root != nil {
		svc.intentMerkleRootHash = root.Hash
	} else {
		svc.intentMerkleRootHash = ""
	}
}

// DeleteIntentByPodID deletes all scheduling intents for a specific pod ID
func (svc *Service) DeleteIntentByPodID(ctx context.Context, podID string) error {
	keysToDelete := []string{}
	svc.schedulingIntentsMap.Range(func(key string, value []*domain.SchedulingIntents) bool {
		if strings.HasPrefix(key, podID+"-") {
			keysToDelete = append(keysToDelete, key)
		}
		return true
	})
	for _, key := range keysToDelete {
		svc.schedulingIntentsMap.Delete(key)
	}
	logger.Logger(ctx).Info().Msgf("Deleted %d scheduling intents for pod ID: %s", len(keysToDelete), podID)
	return nil
}

// DeleteIntentByPID deletes a specific scheduling intent by pod ID and PID
func (svc *Service) DeleteIntentByPID(ctx context.Context, podID string, pid int) error {
	key := fmt.Sprintf("%s-%d", podID, pid)
	svc.schedulingIntentsMap.Delete(key)
	logger.Logger(ctx).Info().Msgf("Deleted scheduling intent for key: %s", key)
	return nil
}

// DeleteAllIntents clears all scheduling intents
func (svc *Service) DeleteAllIntents(ctx context.Context) error {
	keysToDelete := []string{}
	svc.schedulingIntentsMap.Range(func(key string, value []*domain.SchedulingIntents) bool {
		keysToDelete = append(keysToDelete, key)
		return true
	})

	for _, key := range keysToDelete {
		svc.schedulingIntentsMap.Delete(key)
	}

	logger.Logger(ctx).Info().Msgf("Deleted all %d scheduling intents", len(keysToDelete))
	return nil
}
