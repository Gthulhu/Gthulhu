module github.com/Gthulhu/Gthulhu

go 1.22.6

require (
	github.com/Gthulhu/scx_goland_core v0.1.1-0.20250618023044-c10de3c1bc3f
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aquasecurity/libbpfgo v0.8.0-libbpf-1.5 // indirect
	golang.org/x/sys v0.26.0 // indirect
)

replace github.com/aquasecurity/libbpfgo => ./libbpfgo
