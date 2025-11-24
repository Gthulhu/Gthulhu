package parser

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	tracePipePath = "/sys/kernel/debug/tracing/trace_pipe"
)

// TraceParser parses trace_pipe output and extracts nr-gnb and nr-ue PIDs
type TraceParser struct {
	nrGnbRegex *regexp.Regexp
	nrUeRegex  *regexp.Regexp
}

// NewTraceParser creates a new trace parser
func NewTraceParser() *TraceParser {
	// Match patterns like:
	// nr-gnb-1150369 [005] ..s21 34202.967772: bpf_trace_printk: fentry/gtp5g_encap_recv: PID=1150369, TGID=1150348, CPU=5
	// nr-ue-365012 [004] d..31 22353.878390: bpf_trace_printk: stop: pid=365012 (nr-ue) cpu=4
	// Allow optional leading whitespace and tolerate a hyphen or underscore variants
	return &TraceParser{
		nrGnbRegex: regexp.MustCompile(`(?i)^\s*(?:nr[-_]?gnb)[-]?(\d+)`),
		nrUeRegex:  regexp.MustCompile(`(?i)^\s*(?:nr[-_]?ue)[-]?(\d+)`),
	}
}

// ParsePIDFromLine extracts PID from a trace line containing nr-gnb or nr-ue process events
func (p *TraceParser) ParsePIDFromLine(line string) (int, bool) {
	lineLower := strings.ToLower(line)
	
	// Check if line contains nr-gnb, nr-ue or their variants
	hasNrGnb := strings.Contains(lineLower, "nr-gnb") || strings.Contains(lineLower, "nr_gnb")
	hasNrUe := strings.Contains(lineLower, "nr-ue") || strings.Contains(lineLower, "nr_ue")
	
	if !hasNrGnb && !hasNrUe {
		return 0, false
	}

	// Strategy: Extract PIDs in priority order
	// Priority 1: TGID= (thread group ID, the main process)
	// Priority 2: nr-gnb-<PID> (process name format in trace output)
	// Priority 3: pid=<num> (nr-gnb) format
	// Priority 4: PID= field

	// 1) Try TGID first (most reliable for main process)
	tgidRegex := regexp.MustCompile(`(?i)TGID=(\d+)`)
	if tgidMatches := tgidRegex.FindStringSubmatch(line); len(tgidMatches) >= 2 {
		if tgid, err := strconv.Atoi(tgidMatches[1]); err == nil {
			return tgid, true
		}
	}

	// 2) Try process name pattern: nr-gnb-<PID> or nr-ue-<PID>
	if hasNrGnb {
		matches := p.nrGnbRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			if pid, err := strconv.Atoi(matches[1]); err == nil {
				return pid, true
			}
		}
	}
	if hasNrUe {
		matches := p.nrUeRegex.FindStringSubmatch(line)
		if len(matches) >= 2 {
			if pid, err := strconv.Atoi(matches[1]); err == nil {
				return pid, true
			}
		}
	}

	// 3) Try pid=<num> (procname) format with nr-gnb or nr-ue in process name
	pidParenRegex := regexp.MustCompile(`(?i)pid=(\d+)\s*\(([^)]+)\)`)
	if pidMatches := pidParenRegex.FindStringSubmatch(line); len(pidMatches) >= 3 {
		procName := strings.ToLower(pidMatches[2])
		if strings.Contains(procName, "nr-gnb") || strings.Contains(procName, "nr_gnb") ||
			strings.Contains(procName, "nr-ue") || strings.Contains(procName, "nr_ue") {
			if pid, err := strconv.Atoi(pidMatches[1]); err == nil {
				return pid, true
			}
		}
	}

	// 4) Try explicit PID= field (fallback)
	pidRegex := regexp.MustCompile(`(?i)\bPID=(\d+)\b`)
	if pidMatches := pidRegex.FindStringSubmatch(line); len(pidMatches) >= 2 {
		if pid, err := strconv.Atoi(pidMatches[1]); err == nil {
			return pid, true
		}
	}

	return 0, false
}

// StartTailing starts tailing trace_pipe and sends PIDs to the channel
func (p *TraceParser) StartTailing(ctx context.Context, pidChan chan<- int) error {
	// Use cat instead of tail to avoid buffering issues
	cmd := exec.CommandContext(ctx, "cat", tracePipePath)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to get stdout pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start cat command: %w", err)
	}

	log.Printf("Started reading %s", tracePipePath)

	// Read lines in a goroutine
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			line := scanner.Text()
			if pid, ok := p.ParsePIDFromLine(line); ok {
				select {
				case pidChan <- pid:
				case <-ctx.Done():
					return
				}
			}
		}
		if err := scanner.Err(); err != nil {
			log.Printf("Scanner error: %v", err)
		}
	}()

	// Wait for command to finish
	return cmd.Wait()
}
