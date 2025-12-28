module github.com/Gthulhu/Gthulhu

go 1.24.0

toolchain go1.24.2

require (
	github.com/Gthulhu/plugin v1.0.2-0.20251228145509-f9b1fa4c977e
	github.com/Gthulhu/qumun v0.3.2-0.20250105095341-21d6627bc161
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aquasecurity/libbpfgo v0.8.0-libbpf-1.5 // indirect
	github.com/cilium/ebpf v0.20.0 // indirect
	golang.org/x/sys v0.37.0 // indirect
)

replace (
	github.com/Gthulhu/qumun => ./qumun
	github.com/aquasecurity/libbpfgo => ./libbpfgo
)
