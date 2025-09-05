module github.com/Gthulhu/Gthulhu

go 1.22.6

require (
	github.com/Gthulhu/plugin v0.0.0-20250905072935-0410da5d4da9
	github.com/Gthulhu/scx_goland_core v0.3.1-0.20250905073643-f2d179f6324b
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/aquasecurity/libbpfgo v0.8.0-libbpf-1.5 // indirect
	golang.org/x/sys v0.26.0 // indirect
)

replace github.com/aquasecurity/libbpfgo => ./libbpfgo
