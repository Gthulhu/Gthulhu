# Gthulhu Snap Package Installation and Usage

## Overview

The Gthulhu scheduler is available as a snap package for Ubuntu 25.04 and later versions. The snap package provides a self-contained installation with all dependencies included.

## Prerequisites

- Ubuntu 25.04 or later (or compatible distribution with snapd)
- Linux kernel 6.12+ with sched_ext support enabled
- System administrator privileges

### Checking Kernel Compatibility

Before installing, verify your kernel supports sched_ext:

```bash
# Check kernel version
uname -r

# Check if sched_ext is available
ls /sys/kernel/sched_ext/
```

If the `/sys/kernel/sched_ext/` directory doesn't exist, your kernel doesn't have sched_ext support enabled.

## Installation

### From Snap Store

```bash
# Install from the stable channel
sudo snap install gthulhu

# Or install from edge channel for latest development version
sudo snap install gthulhu --edge
```

### Local Installation (Development)

```bash
# Build and install locally
snapcraft
sudo snap install --dangerous gthulhu_*.snap
```

## Usage

### Basic Usage

```bash
# Run the scheduler (requires root privileges)
sudo snap run gthulhu

# Run with custom configuration
sudo snap run gthulhu -config /path/to/config.yaml
```

### Configuration

The snap creates a default configuration file at `~/snap/gthulhu/current/config.yaml` on first run. You can customize this file to adjust scheduler parameters:

```yaml
scheduler:
  slice_ns_default: 5000000  # 5ms default time slice
  slice_ns_min: 500000       # 0.5ms minimum time slice

api:
  enabled: true
  url: http://127.0.0.1:8080
  interval: 5

debug: true
early_processing: false
builtin_idle: false
```

### Monitoring

```bash
# Check scheduler status
sudo bpftool prog list | grep gthulhu

# View BPF trace output
sudo cat /sys/kernel/debug/tracing/trace_pipe

# Check snap logs
snap logs gthulhu
```

## Permissions and Security

The Gthulhu snap uses `classic` confinement due to its need for deep system integration. It requires the following system access:

- **kernel-module-control**: For loading BPF programs
- **system-observe**: For system monitoring
- **hardware-observe**: For CPU topology awareness
- **process-control**: For task scheduling control

These permissions are automatically granted when installing with `classic` confinement.

## Troubleshooting

### Common Issues

1. **Permission Denied**: Ensure you're running with `sudo`
2. **Kernel Not Supported**: Upgrade to kernel 6.12+ with sched_ext
3. **BPF Load Failed**: Check if CONFIG_SCHED_CLASS_EXT=y in kernel config

### Getting Help

```bash
# View snap information
snap info gthulhu

# Check snap connections
snap connections gthulhu

# View detailed logs
journalctl -u snap.gthulhu.gthulhu
```

## Development and Contributing

The snap configuration is maintained in the main repository. To contribute:

1. Fork the repository
2. Modify `snapcraft.yaml` or related files
3. Test with `snapcraft --debug`
4. Submit a pull request

### Building from Source

```bash
# Clone repository
git clone https://github.com/Gthulhu/Gthulhu.git
cd Gthulhu

# Build snap
snapcraft

# Install locally
sudo snap install --dangerous gthulhu_*.snap
```

## Channels and Releases

- **stable**: Production-ready releases
- **candidate**: Release candidates, tested but not yet stable
- **beta**: Beta releases for testing
- **edge**: Latest development builds

Switch between channels:

```bash
sudo snap refresh gthulhu --channel=edge
```

## Support

For snap-specific issues, please report them in the [GitHub repository](https://github.com/Gthulhu/Gthulhu/issues) with the "snap" label.