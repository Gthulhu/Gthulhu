package migration

import (
	"log"
	"time"
)

// Expiremental feature
// TaskMigrationManager manages task migration to prevent frequent migrations
type TaskMigrationManager struct {
	factor                int
	factorCount           int
	cpuNum                int
	sumExecutionMap       map[int]map[int32]uint64
	workloadPerCPU        map[int32]uint64
	preventMigrationCount uint64
	current               int
	timeStamp             time.Time
}

func NewTaskMigrationManger() *TaskMigrationManager {
	m := &TaskMigrationManager{
		factor:                10,
		factorCount:           1,
		cpuNum:                20,
		sumExecutionMap:       make(map[int]map[int32]uint64),
		workloadPerCPU:        make(map[int32]uint64),
		preventMigrationCount: 0,
		current:               0,
		timeStamp:             time.Now(),
	}
	return m
}

func (m *TaskMigrationManager) Rotate() {
	m.sumExecutionMap[m.factorCount%m.factor] = make(map[int32]uint64)
	m.workloadPerCPU = map[int32]uint64{}
	m.current = m.factorCount % m.factor
	for i := 0; i < m.cpuNum; i++ {
		for j := 0; j < m.factor; j++ {
			weightFactor := m.factor
			if j > m.current {
				weightFactor -= (m.factor - m.current - 1 + j - m.current)
				if weightFactor < 0 {
					weightFactor = -weightFactor
				}
			} else if j < m.current {
				weightFactor -= (m.current - j)
			} else {
				weightFactor = 0
			}
			m.workloadPerCPU[int32(i)] += m.sumExecutionMap[j][int32(i)] * uint64(weightFactor)
		}
	}
}

func (m *TaskMigrationManager) addExecutionTime(cpu int32, executionTime uint64) {
	if m.sumExecutionMap[m.current] == nil {
		m.sumExecutionMap[m.current] = make(map[int32]uint64)
	}
	m.sumExecutionMap[m.current][cpu] += executionTime
}

func (m *TaskMigrationManager) needMigrate(cpu, prevCpu int32) bool {
	need := m.workloadPerCPU[cpu] < m.workloadPerCPU[prevCpu]*9/10
	need = need && m.factorCount > m.factor
	if !need {
		m.preventMigrationCount++
	}
	return need
}

func (m *TaskMigrationManager) Do(cpu int32, prevCpu int32, executionTime uint64) bool {
	now := time.Now()
	if now.Sub(m.timeStamp) > time.Second {
		m.Rotate()
		m.timeStamp = now
	}
	m.addExecutionTime(cpu, executionTime)
	return m.needMigrate(cpu, prevCpu)
}

func (m *TaskMigrationManager) Done() {
	log.Printf("Prevent migration count: %d", m.preventMigrationCount)
}
