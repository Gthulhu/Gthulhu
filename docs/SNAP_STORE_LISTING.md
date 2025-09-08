# Snap Store Listing Information

## Name
gthulhu

## Summary
Linux sched_ext scheduler for cloud-native workloads

## Description
Gthulhu is a high-performance Linux scheduler extension that optimizes cloud-native workloads using eBPF for kernel-level scheduling and Go for user-space policy implementation.

### Key Features
- Virtual runtime (vruntime) based scheduling algorithm
- Latency-sensitive task prioritization
- Dynamic time slice adjustment based on workload characteristics  
- CPU topology aware task placement for NUMA optimization
- Automatic idle CPU selection for power efficiency
- Real-time API for dynamic scheduling policy updates

### Architecture
This is a hybrid scheduler architecture that combines:
- **BPF Component**: Implements low-level sched-ext kernel interface
- **Go Scheduler**: User-space scheduling policy implementation
- **API Server**: REST API for runtime configuration

### Requirements
- Linux kernel 6.12+ with CONFIG_SCHED_CLASS_EXT=y
- System administrator privileges (uses classic confinement)
- Compatible with Ubuntu 25.04+, other Linux distributions with snapd

### Safety Notice
⚠️ This scheduler requires classic confinement and root privileges to load BPF programs and modify kernel scheduling behavior. Use with caution in production environments and ensure you understand the implications of changing system schedulers.

### Usage
```bash
sudo snap install gthulhu
sudo snap run gthulhu
```

For detailed documentation, visit: https://github.com/Gthulhu/Gthulhu

## Categories
- development
- system

## License
GPL-2.0

## Website
https://github.com/Gthulhu/Gthulhu

## Contact
https://github.com/Gthulhu/Gthulhu/issues

## Keywords
scheduler, bpf, ebpf, kernel, performance, cloud-native, sched-ext, linux