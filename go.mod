module github.com/Gthulhu/Gthulhu

go 1.22.6

require github.com/Gthulhu/scx_goland_core v0.0.3-0.20250609181621-b90f94cc5ca0

require (
	github.com/aquasecurity/libbpfgo v0.8.0-libbpf-1.5 // indirect
	golang.org/x/sys v0.26.0 // indirect
)

replace github.com/aquasecurity/libbpfgo => ./libbpfgo
