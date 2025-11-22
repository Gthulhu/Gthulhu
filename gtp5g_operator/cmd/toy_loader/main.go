package main

import (
	"encoding/binary"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	libbpf "github.com/aquasecurity/libbpfgo"
)

// Go struct must match C struct layout (packed with natural alignment)
type Event struct {
	TsNs    uint64
	Cpu     uint32
	Pid     uint32
	Tgid    uint32
	Comm    [16]byte
	SrcIP   uint32
	DstIP   uint32
	SrcPort uint16
	DstPort uint16
	PktLen  uint32
	Teid    uint32
	FuncID  uint32
}

func decodeEvent(data []byte) Event {
	var e Event
	// Note: binary.LittleEndian used since BPF on x86 is little-endian
	// Fill fields in same order
	e.TsNs = binary.LittleEndian.Uint64(data[0:8])
	e.Cpu = binary.LittleEndian.Uint32(data[8:12])
	e.Pid = binary.LittleEndian.Uint32(data[12:16])
	e.Tgid = binary.LittleEndian.Uint32(data[16:20])
	copy(e.Comm[:], data[20:36])
	e.SrcIP = binary.LittleEndian.Uint32(data[36:40])
	e.DstIP = binary.LittleEndian.Uint32(data[40:44])
	e.SrcPort = binary.LittleEndian.Uint16(data[44:46])
	e.DstPort = binary.LittleEndian.Uint16(data[46:48])
	e.PktLen = binary.LittleEndian.Uint32(data[48:52])
	e.Teid = binary.LittleEndian.Uint32(data[52:56])
	e.FuncID = binary.LittleEndian.Uint32(data[56:60])
	return e
}

func main() {
	if len(os.Args) != 2 {
		fmt.Printf("Usage: %s <bpf_object.o>\n", os.Args[0])
		os.Exit(1)
	}
	obj := os.Args[1]

	eventsChan := make(chan []byte)

	module, err := libbpf.NewModuleFromFile(obj)
	if err != nil {
		panic(err)
	}
	defer module.Close()

	if err := module.BPFLoadObject(); err != nil {
		panic(err)
	}

	fmt.Println("BPF object loaded successfully")

	ringbuf, err := module.InitRingBuf("events", eventsChan)
	if err != nil {
		panic(err)
	}

	fmt.Println("Ring buffer initialized")

	// Start ring buffer
	ringbuf.Start()

	fmt.Println("Ring buffer started, listening for events...")

	// Read from channel in a goroutine
	go func() {
		for data := range eventsChan {
			if len(data) < 60 {
				fmt.Printf("event too small: %d bytes\n", len(data))
				continue
			}
			e := decodeEvent(data)
			fmt.Printf("event: ts=%d pid=%d tgid=%d comm=%s func=%d pktlen=%d\n", e.TsNs, e.Pid, e.Tgid, string(e.Comm[:]), e.FuncID, e.PktLen)
		}
	}()

	// trap signals to stop
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	<-sig

	ringbuf.Stop()
	module.Close()
	fmt.Println("exiting")
}
