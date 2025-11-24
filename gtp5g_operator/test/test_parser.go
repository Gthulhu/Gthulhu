package main

import (
	"fmt"
	"log"

	"github.com/Gthulhu/Gthulhu/gtp5g_operator/pkg/parser"
)

func main() {
	log.Println("=== Testing Trace Parser ===")

	// Test data
	testLines := []string{
		// Valid nr-gnb lines
		"nr-gnb-12345 [001] d..41 18498.255587: bpf_trace_printk: enqueue: pid=12345 (nr-gnb)",
		"           <...>-943015  [004] d..71 18498.255497: bpf_trace_printk: enqueue: pid=942021 (gtp5g_operator)",
		"nr-gnb-67890 [002] d..31 18498.255561: bpf_trace_printk: start: pid=67890 (nr-gnb) cpu=1",
		// Invalid lines
		"           <...>-942013  [001] d..51 18498.255572: bpf_trace_printk: enqueue: pid=942014 (gtp5g_operator)",
		"some random text",
		"",
	}

	p := parser.NewTraceParser()

	successCount := 0
	for i, line := range testLines {
		pid, ok := p.ParsePIDFromLine(line)
		if ok {
			successCount++
			log.Printf("✅ Line %d: Extracted PID %d", i+1, pid)
		} else {
			log.Printf("❌ Line %d: No PID found (expected for non-nr-gnb lines)", i+1)
		}
	}

	fmt.Println()
	log.Printf("=== Results ===")
	log.Printf("Total lines tested: %d", len(testLines))
	log.Printf("PIDs extracted: %d", successCount)
	
	if successCount == 2 {
		log.Println("✅ Parser Test Passed!")
	} else {
		log.Printf("⚠️  Expected 2 PIDs, got %d", successCount)
	}
}
