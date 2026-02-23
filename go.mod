module github.com/Gthulhu/Gthulhu

go 1.24.0

toolchain go1.24.2

require (
	github.com/Gthulhu/plugin v1.1.1-0.20260223021833-8f174def9e15
	gopkg.in/yaml.v3 v3.0.1
)

replace (
	github.com/Gthulhu/qumun => ./qumun
	github.com/aquasecurity/libbpfgo => ./libbpfgo
)
