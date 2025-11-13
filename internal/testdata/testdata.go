package testdata

import (
	"embed"
)

//go:embed crds.yaml
var CRDsYAML []byte

//go:embed different-gv.yaml
var DifferentGVYAML []byte

//go:embed resource.go.generated.txt
var ResourceGoGenerated string

//go:embed samplerefs.yaml
var SampleRefsYAML []byte

//go:embed v1
var V1 embed.FS

//go:embed refs
var Refs embed.FS
