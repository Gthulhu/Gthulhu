package rest

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	featureCount            = 6
	ewmaShortAlpha          = 0.3
	ewmaLongAlpha           = 0.05
	ewmaEpsilon             = 1e-9
	driftThreshold          = 1.5
	driftConfirmThreshold   = 3
	warmupMinSamples        = 10
	stableMinSamples        = 30
	maxClusterBufferSamples = 1000
)

type PodPhase string

const (
	PodPhaseColdStart     PodPhase = "cold_start"
	PodPhaseWarmingUp     PodPhase = "warming_up"
	PodPhaseStable        PodPhase = "stable"
	PodPhaseDrifting      PodPhase = "drifting"
	PodPhaseTransitioning PodPhase = "transitioning"
)

type featureVector [featureCount]float64

type classificationInput struct {
	Timestamp int64
	Namespace string
	Pod       string
	Node      string
	Metrics   metricsPayload
}

type metricsPayload struct {
	VolCtxSW   uint64 `json:"vol_ctx_sw"`
	InvolCtxSW uint64 `json:"invol_ctx_sw"`
	CPUTime    uint64 `json:"cpu_time"`
	WaitTime   uint64 `json:"wait_time"`
	RunCount   uint64 `json:"run_count"`
	SMTMigr    uint64 `json:"smt_migr"`
	L3Migr     uint64 `json:"l3_migr"`
	NUMAMigr   uint64 `json:"numa_migr"`
}

type ewmaState struct {
	initialized bool
	shortMean   featureVector
	shortVar    featureVector
	longMean    featureVector
	longVar     featureVector
	updateCount int
}

func (e *ewmaState) Normalize(x featureVector) featureVector {
	if !e.initialized {
		return x
	}
	var z featureVector
	for i := 0; i < featureCount; i++ {
		z[i] = (x[i] - e.longMean[i]) / math.Sqrt(e.longVar[i]+ewmaEpsilon)
	}
	return z
}

func (e *ewmaState) Update(x featureVector) {
	if !e.initialized {
		e.shortMean = x
		e.longMean = x
		for i := 0; i < featureCount; i++ {
			e.shortVar[i] = 1e-6
			e.longVar[i] = 1e-6
		}
		e.initialized = true
		e.updateCount = 1
		return
	}

	for i := 0; i < featureCount; i++ {
		shortDiff := x[i] - e.shortMean[i]
		e.shortMean[i] = e.shortMean[i] + ewmaShortAlpha*shortDiff
		e.shortVar[i] = (1 - ewmaShortAlpha) * (e.shortVar[i] + ewmaShortAlpha*shortDiff*shortDiff)

		longDiff := x[i] - e.longMean[i]
		e.longMean[i] = e.longMean[i] + ewmaLongAlpha*longDiff
		e.longVar[i] = (1 - ewmaLongAlpha) * (e.longVar[i] + ewmaLongAlpha*longDiff*longDiff)
	}
	e.updateCount++
}

func (e *ewmaState) DriftScore() float64 {
	if !e.initialized {
		return 0
	}
	total := 0.0
	for i := 0; i < featureCount; i++ {
		total += math.Abs(e.shortMean[i]-e.longMean[i]) / math.Sqrt(e.longVar[i]+ewmaEpsilon)
	}
	return total / featureCount
}

type tieredSummary struct {
	Mean      featureVector
	Std       featureVector
	Timestamp int64
	Count     int
}

type tieredBuffer struct {
	raw      []tieredSummary
	tier1    []tieredSummary
	tier2    []tieredSummary
	tier3    []tieredSummary
	pending1 []tieredSummary
	pending2 []tieredSummary
	pending3 []tieredSummary
}

func (b *tieredBuffer) Push(v featureVector, ts int64) {
	rawSample := tieredSummary{Mean: v, Timestamp: ts, Count: 1}
	b.raw = appendBoundedSummary(b.raw, rawSample, 10)
	b.pending1 = append(b.pending1, rawSample)
	if len(b.pending1) >= 2 {
		s1 := summarizeTiered(b.pending1)
		b.pending1 = b.pending1[:0]
		b.tier1 = appendBoundedSummary(b.tier1, s1, 10)
		b.pending2 = append(b.pending2, s1)
	}

	if len(b.pending2) >= 5 {
		s2 := summarizeTiered(b.pending2)
		b.pending2 = b.pending2[:0]
		b.tier2 = appendBoundedSummary(b.tier2, s2, 6)
		b.pending3 = append(b.pending3, s2)
	}

	if len(b.pending3) >= 6 {
		s3 := summarizeTiered(b.pending3)
		b.pending3 = b.pending3[:0]
		b.tier3 = appendBoundedSummary(b.tier3, s3, 6)
	}
}

func appendBoundedSummary(items []tieredSummary, item tieredSummary, max int) []tieredSummary {
	if len(items) >= max {
		copy(items, items[1:])
		items[len(items)-1] = item
		return items
	}
	return append(items, item)
}

func summarizeTiered(items []tieredSummary) tieredSummary {
	if len(items) == 0 {
		return tieredSummary{}
	}
	var out tieredSummary
	totalCount := 0
	for _, item := range items {
		totalCount += item.Count
		for i := 0; i < featureCount; i++ {
			out.Mean[i] += item.Mean[i] * float64(item.Count)
		}
		if item.Timestamp > out.Timestamp {
			out.Timestamp = item.Timestamp
		}
	}
	if totalCount == 0 {
		totalCount = len(items)
	}
	for i := 0; i < featureCount; i++ {
		out.Mean[i] /= float64(totalCount)
	}
	for _, item := range items {
		for i := 0; i < featureCount; i++ {
			d := item.Mean[i] - out.Mean[i]
			out.Std[i] += d * d
		}
	}
	for i := 0; i < featureCount; i++ {
		out.Std[i] = math.Sqrt(out.Std[i] / float64(len(items)))
	}
	out.Count = totalCount
	return out
}

type adaptiveClusteringModel struct {
	nClusters        int
	dataBuffer       []featureVector
	clusterCenters   []featureVector
	clusterSemantics map[int][]string
	isFitted         bool
}

func newAdaptiveClusteringModel(n int) *adaptiveClusteringModel {
	return &adaptiveClusteringModel{
		nClusters:        n,
		clusterSemantics: map[int][]string{},
		dataBuffer:       make([]featureVector, 0, maxClusterBufferSamples),
	}
}

func (m *adaptiveClusteringModel) PartialFit(x featureVector) {
	if len(m.dataBuffer) >= maxClusterBufferSamples {
		copy(m.dataBuffer, m.dataBuffer[1:])
		m.dataBuffer[len(m.dataBuffer)-1] = x
	} else {
		m.dataBuffer = append(m.dataBuffer, x)
	}

	if !m.isFitted || len(m.clusterCenters) == 0 {
		return
	}
	cid := m.Predict(x)
	if cid < 0 || cid >= len(m.clusterCenters) {
		return
	}
	const eta = 0.05
	for i := 0; i < featureCount; i++ {
		m.clusterCenters[cid][i] += eta * (x[i] - m.clusterCenters[cid][i])
	}
}

func (m *adaptiveClusteringModel) SnapshotAndRelabel() {
	if len(m.dataBuffer) < 2 {
		return
	}
	k := m.nClusters
	if k > len(m.dataBuffer) {
		k = len(m.dataBuffer)
	}
	if k <= 0 {
		return
	}
	centers := kmeans(m.dataBuffer, k, 10)
	if len(centers) == 0 {
		return
	}
	m.clusterCenters = centers
	m.clusterSemantics = inferSemantics(centers)
	m.isFitted = true
}

func (m *adaptiveClusteringModel) Predict(x featureVector) int {
	if !m.isFitted || len(m.clusterCenters) == 0 {
		return -1
	}
	bestIdx := 0
	bestDist := math.MaxFloat64
	for i, c := range m.clusterCenters {
		d := squaredDistance(x, c)
		if d < bestDist {
			bestDist = d
			bestIdx = i
		}
	}
	return bestIdx
}

func kmeans(data []featureVector, k int, iter int) []featureVector {
	if len(data) == 0 || k <= 0 {
		return nil
	}
	if k > len(data) {
		k = len(data)
	}
	centers := make([]featureVector, k)
	for i := 0; i < k; i++ {
		centers[i] = data[i]
	}

	assign := make([]int, len(data))
	for t := 0; t < iter; t++ {
		changed := false
		for i, x := range data {
			best := 0
			bestDist := math.MaxFloat64
			for ci, c := range centers {
				d := squaredDistance(x, c)
				if d < bestDist {
					bestDist = d
					best = ci
				}
			}
			if assign[i] != best {
				assign[i] = best
				changed = true
			}
		}

		if !changed && t > 0 {
			break
		}

		sums := make([]featureVector, k)
		counts := make([]int, k)
		for i, x := range data {
			cid := assign[i]
			counts[cid]++
			for fi := 0; fi < featureCount; fi++ {
				sums[cid][fi] += x[fi]
			}
		}
		for ci := 0; ci < k; ci++ {
			if counts[ci] == 0 {
				continue
			}
			for fi := 0; fi < featureCount; fi++ {
				centers[ci][fi] = sums[ci][fi] / float64(counts[ci])
			}
		}
	}
	return centers
}

func inferSemantics(centers []featureVector) map[int][]string {
	out := map[int][]string{}
	if len(centers) == 0 {
		return out
	}

	maxIdx := make([]int, featureCount)
	for f := 0; f < featureCount; f++ {
		best := 0
		bestV := centers[0][f]
		for i := 1; i < len(centers); i++ {
			if centers[i][f] > bestV {
				bestV = centers[i][f]
				best = i
			}
		}
		maxIdx[f] = best
	}

	for i := range centers {
		tags := make([]string, 0, 3)
		if maxIdx[2] == i {
			tags = append(tags, "cpu_heavy")
		}
		if maxIdx[1] == i {
			tags = append(tags, "needs_higher_priority")
		}
		if maxIdx[0] == i {
			tags = append(tags, "interactive")
		}
		if maxIdx[4] == i {
			tags = append(tags, "cache_unfriendly")
		}
		if maxIdx[5] == i {
			tags = append(tags, "numa_unfriendly")
		}
		if maxIdx[3] == i {
			tags = append(tags, "scheduling_latency")
		}
		if len(tags) == 0 {
			tags = []string{"balanced"}
		}
		out[i] = tags
	}
	return out
}

func squaredDistance(a, b featureVector) float64 {
	total := 0.0
	for i := 0; i < featureCount; i++ {
		d := a[i] - b[i]
		total += d * d
	}
	return total
}

type podState struct {
	namespace       string
	pod             string
	node            string
	ewma            ewmaState
	buffer          tieredBuffer
	phase           PodPhase
	currentCluster  int
	previousCluster int
	driftConfirmed  int
	lastDriftScore  float64
	currentTypes    []string
	previousTypes   []string
	lastTimestamp   int64
}

func newPodState(namespace, pod string) *podState {
	return &podState{
		namespace:       namespace,
		pod:             pod,
		phase:           PodPhaseColdStart,
		currentCluster:  -1,
		previousCluster: -1,
		currentTypes:    []string{"collecting"},
		previousTypes:   []string{},
	}
}

func (p *podState) Update(ts int64, node string, x featureVector, model *adaptiveClusteringModel) {
	p.node = node
	p.lastTimestamp = ts

	norm := p.ewma.Normalize(x)
	p.ewma.Update(x)
	p.buffer.Push(x, ts)
	model.PartialFit(norm)

	if p.ewma.updateCount < warmupMinSamples {
		p.phase = PodPhaseColdStart
		p.currentTypes = []string{"collecting"}
		p.lastDriftScore = p.ewma.DriftScore()
		return
	}

	if p.ewma.updateCount < stableMinSamples {
		p.phase = PodPhaseWarmingUp
		if !model.isFitted && len(model.dataBuffer) >= stableMinSamples {
			model.SnapshotAndRelabel()
		}
		if model.isFitted {
			cid := model.Predict(norm)
			if cid >= 0 {
				p.currentCluster = cid
				p.currentTypes = cloneStringSlice(model.clusterSemantics[cid])
			}
		}
		p.lastDriftScore = p.ewma.DriftScore()
		return
	}

	if !model.isFitted {
		model.SnapshotAndRelabel()
	}

	drift := p.ewma.DriftScore()
	p.lastDriftScore = drift
	if drift > driftThreshold {
		p.driftConfirmed++
		if p.driftConfirmed >= driftConfirmThreshold {
			p.phase = PodPhaseTransitioning
			p.previousCluster = p.currentCluster
			if p.previousCluster >= 0 {
				p.previousTypes = cloneStringSlice(model.clusterSemantics[p.previousCluster])
			}
			model.SnapshotAndRelabel()
			newCID := model.Predict(norm)
			p.currentCluster = newCID
			p.currentTypes = cloneStringSlice(model.clusterSemantics[newCID])
			p.driftConfirmed = 0
			return
		}
		p.phase = PodPhaseDrifting
		return
	}

	p.driftConfirmed = 0
	p.phase = PodPhaseStable
	cid := model.Predict(norm)
	if cid >= 0 {
		p.currentCluster = cid
		p.currentTypes = cloneStringSlice(model.clusterSemantics[cid])
	}
}

func (p *podState) Confidence() float64 {
	switch p.phase {
	case PodPhaseColdStart:
		return 0.0
	case PodPhaseWarmingUp:
		v := 0.2 + float64(p.ewma.updateCount)/float64(stableMinSamples)*0.4
		if v > 0.6 {
			return 0.6
		}
		return v
	case PodPhaseDrifting:
		return 0.55
	case PodPhaseTransitioning:
		return 0.7
	default:
		v := 0.95 - math.Min(p.lastDriftScore, 1.5)*0.1
		if v < 0.75 {
			return 0.75
		}
		return v
	}
}

func (p *podState) Recommendation() (action, priorityClass, reason string) {
	tags := toTagSet(p.currentTypes)
	switch {
	case tags["needs_higher_priority"]:
		return "raise_priority", "high-priority", "Involuntary context switch ratio remains high and indicates CPU contention."
	case tags["cpu_heavy"]:
		return "increase_cpu_limit", "default", "CPU per run is dominant in current cluster profile."
	case tags["numa_unfriendly"] || tags["cache_unfriendly"]:
		return "enable_cpu_pinning", "default", "Cross-core/NUMA migration pattern suggests topology-unaware placement."
	default:
		return "keep_current", "default", "Current profile is stable and does not require immediate tuning."
	}
}

func toTagSet(tags []string) map[string]bool {
	m := map[string]bool{}
	for _, t := range tags {
		m[t] = true
	}
	return m
}

func cloneStringSlice(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

type classifyDrift struct {
	DriftScore            float64 `json:"drift_score"`
	DriftConfirmedPeriods int     `json:"drift_confirmed_periods"`
}

type classifyProfile struct {
	ShortTerm        map[string]float64 `json:"short_term"`
	LongTermBaseline map[string]float64 `json:"long_term_baseline"`
}

type classifyRecommendation struct {
	Action        string `json:"action"`
	PriorityClass string `json:"priority_class"`
	Reason        string `json:"reason"`
}

type classifyResult struct {
	CurrentType  []string `json:"current_type"`
	PreviousType []string `json:"previous_type"`
	Confidence   float64  `json:"confidence"`
}

type classifyResponseItem struct {
	Pod            string                 `json:"pod"`
	Namespace      string                 `json:"namespace"`
	Node           string                 `json:"node,omitempty"`
	Phase          PodPhase               `json:"phase"`
	Classification classifyResult         `json:"classification"`
	Drift          classifyDrift          `json:"drift"`
	Profile        classifyProfile        `json:"profile"`
	Recommendation classifyRecommendation `json:"recommendation"`
	UpdatedAt      int64                  `json:"updated_at"`
}

type listClassifyResponse struct {
	Items []*classifyResponseItem `json:"items"`
}

type AdaptiveClassifier struct {
	mu    sync.RWMutex
	pods  map[string]*podState
	model *adaptiveClusteringModel
}

func NewAdaptiveClassifier(nClusters int) *AdaptiveClassifier {
	if nClusters <= 0 {
		nClusters = 5
	}
	return &AdaptiveClassifier{
		pods:  map[string]*podState{},
		model: newAdaptiveClusteringModel(nClusters),
	}
}

func (c *AdaptiveClassifier) Ingest(input classificationInput) *classifyResponseItem {
	c.mu.Lock()
	defer c.mu.Unlock()

	ts := input.Timestamp
	if ts <= 0 {
		ts = time.Now().Unix()
	}
	key := input.Namespace + "/" + input.Pod
	st, ok := c.pods[key]
	if !ok {
		st = newPodState(input.Namespace, input.Pod)
		c.pods[key] = st
	}
	fv := computeFeatures(input.Metrics)
	st.Update(ts, input.Node, fv, c.model)
	return buildClassifyItem(st)
}

func (c *AdaptiveClassifier) Get(namespace, pod string) (*classifyResponseItem, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	st, ok := c.pods[namespace+"/"+pod]
	if !ok {
		return nil, false
	}
	return buildClassifyItem(st), true
}

func (c *AdaptiveClassifier) List(namespace string, phase PodPhase, t string) []*classifyResponseItem {
	c.mu.RLock()
	defer c.mu.RUnlock()

	items := make([]*classifyResponseItem, 0, len(c.pods))
	for _, st := range c.pods {
		if namespace != "" && st.namespace != namespace {
			continue
		}
		if phase != "" && st.phase != phase {
			continue
		}
		if t != "" && !containsTag(st.currentTypes, t) {
			continue
		}
		items = append(items, buildClassifyItem(st))
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Namespace != items[j].Namespace {
			return items[i].Namespace < items[j].Namespace
		}
		return items[i].Pod < items[j].Pod
	})
	return items
}

func containsTag(tags []string, target string) bool {
	target = strings.TrimSpace(strings.ToLower(target))
	for _, t := range tags {
		if strings.ToLower(t) == target {
			return true
		}
	}
	return false
}

func buildClassifyItem(st *podState) *classifyResponseItem {
	action, pri, reason := st.Recommendation()
	return &classifyResponseItem{
		Pod:       st.pod,
		Namespace: st.namespace,
		Node:      st.node,
		Phase:     st.phase,
		Classification: classifyResult{
			CurrentType:  cloneStringSlice(st.currentTypes),
			PreviousType: cloneStringSlice(st.previousTypes),
			Confidence:   st.Confidence(),
		},
		Drift: classifyDrift{
			DriftScore:            st.lastDriftScore,
			DriftConfirmedPeriods: st.driftConfirmed,
		},
		Profile: classifyProfile{
			ShortTerm: map[string]float64{
				"vol_ctx_ratio":      st.ewma.shortMean[0],
				"invol_ctx_ratio":    st.ewma.shortMean[1],
				"cpu_per_run":        st.ewma.shortMean[2],
				"wait_ratio":         st.ewma.shortMean[3],
				"cache_migr_per_run": st.ewma.shortMean[4],
				"numa_migr_ratio":    st.ewma.shortMean[5],
			},
			LongTermBaseline: map[string]float64{
				"vol_ctx_ratio":      st.ewma.longMean[0],
				"invol_ctx_ratio":    st.ewma.longMean[1],
				"cpu_per_run":        st.ewma.longMean[2],
				"wait_ratio":         st.ewma.longMean[3],
				"cache_migr_per_run": st.ewma.longMean[4],
				"numa_migr_ratio":    st.ewma.longMean[5],
			},
		},
		Recommendation: classifyRecommendation{
			Action:        action,
			PriorityClass: pri,
			Reason:        reason,
		},
		UpdatedAt: st.lastTimestamp,
	}
}

func computeFeatures(m metricsPayload) featureVector {
	run := math.Max(float64(m.RunCount), 1.0)
	cpu := float64(m.CPUTime)
	wait := float64(m.WaitTime)
	crossCache := float64(m.L3Migr + m.NUMAMigr)

	return featureVector{
		float64(m.VolCtxSW) / run,
		float64(m.InvolCtxSW) / run,
		cpu / run,
		wait / math.Max(cpu+wait, 1.0),
		crossCache / run,
		float64(m.NUMAMigr) / math.Max(crossCache, 1e-9),
	}
}
