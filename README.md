# CRD2Go

CRD2Go is a simple tool to generate your Go struct types off existing Kubernetes CRDs. This is best suited for development workflows in which the CRDs might be available before the Go code as, for instance might happen if the CRD code is auto generated from an OpenAPI spec.

## Usage

You can use this as a Go tool directly in your Go projects with a simple command:

```shell
go get -tool github.com/crd2go/crd2go
```

Then just run it as a CLI tool:

```shell
go tool crd2go -h
Usage of ./crd2go:
  -config string
    	YAML file with the CRD2Go config (default "crd2go.yaml")
  -input string
    	input YAML to process
  -output string
    	output directory to produce source code to
```

Main arguments:
- **config** is the name of a YAML file containing all tool settings.
- **input** is the name of a YAML file with one or more CRDs to generate Go code from.
- **output** is the directory where the Go types for the CRDs should be generated.

The *input* and *output* CLI args override the values in the config file.

### Configuration

Refer to the sample [crd2go.yaml](crd2go.yaml) file as a full sample configuration file.

Extra configuration settings:
- *skipList* is an array listing CRD Kinds to be skipped from generation.
- *reserved* is an array of type names that are not to be used in code generation.
- *renames* are key - value pairs that specify how each key typename should be renamed to the given value when generated.
- *imports* associate a type name with an import path and alias, so that an existing Go type is used instead of further expanding a CRD defined type in the generated code.
- *deepCopy* controls how deep copy generation is handled:
  - *deepCopy.generate* specifies whether or not to attempt to run `controller-gen` for deep copy generation. It defaults to `auto`, meaning try to run `controller-gen` when available in the PATH.
  - *deepCopy.controllerGenPath* can be used to give a custom path to the `controller-gen` binary.
