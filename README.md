# CRD2Go

CRD2Go is a simple tool to generate your Go struct types off existing CRDs. This is best suited for development workflows in which the CRDs might be avbailable before the Go code as, for insatnce might happeni fthe CRD code is auto generated from an OpenAPi spec or similar.

## Usage

You can this as a tool directly in your Go projects with a simple command:

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
- **input** is the name of a YAML file with one or more CRDs to be converted to Go code.
- **output** is the directory where the Go types for the CRds should be generated.

The *input* and *output* CLI args oevrride the values in the config file.

### Configuration

Refer to the sample [crd2go.yaml](crd2go.yaml) file as a full sample configuration file.

Extra configuration settings:
- *skipList* is an array listing CRD Kinds to be skipped from generation, if any.
- *reserved* is an array of type names that are not to be produced by code generation.
- *renames* are key - value pairs that specify how each key typename should be renamed to the given value when generated.
- *imports* associate a type name with an import path and alias, so that an existing Go type is used instead of further expanding such type in the generated code.
- *deepCopy* controls how deep copy generation is handled.
  - *deepCopy.generate* specifies whether or not to attempt to run `controller-gen` for deep copy generation. It defaults to `auto`, meaning try to run `controller-gen` when available in the PATH.
  - *deepCopy.controllerGenPath* can be used to give a custom path to the `controller-gen` binary.
