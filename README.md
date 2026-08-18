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
  -gv string
    	Group Version (e.g 'gen.example.com/v1') to generate from.
  -input string
    	input YAML to process
  -output string
    	output directory to produce source code to
```

Main arguments:
- **config** is the name of a YAML file containing all tool settings.
- **gv** is an optional parameter to explicitly select a Group and Version to be generated. When this argument is set, any entres from other group versions will be skipped. When is it not set, the assumption is that all kinds are of the same group version, or the generation fails otherwise.
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
- *fileNameFormat* used to name the generated files. `%s` is replaced by the lowercased Kind and the .go extension is always appended. When set, the format must contain `%s`, otherwise all Kinds would target the same file. When unset, files are named after the Kind alone:

```yaml
fileNameFormat: "%s_types"   #  filename output becomes mycrd_types.go, doc_types.go and groupversion_info_types.go

**Deprecated**: *deepCopy.generate* and *applyConfiguration* as top-level settings are deprecated; use the `gen-deepcopy` and `gen-applyconfiguration` plugins instead. The legacy fields still work but emit a warning to stderr. Setting a legacy field alongside its equivalent plugin is an error.

> **Breaking change — deepcopy default flipped to off.** Previous releases treated an unset `deepCopy.generate` as enabled, so an empty config still emitted `+k8s:deepcopy-gen` markers. Starting with this release, deepcopy markers are only emitted when the `gen-deepcopy` plugin is listed under `plugins` (or the deprecated `deepCopy.generate: true` is set, which is auto-translated). Configs that relied on the implicit default will silently stop generating deepcopy markers — add `gen-deepcopy` to the `plugins` list to restore the previous behavior.

### Plugins

Optional plugins extend the generated code on a per-CRD basis. Enable them in `crd2go.yaml` under `plugins`:

```yaml
plugins:
  - name: <plugin-name>
    options:            # optional, plugin-specific; typed per plugin
      key: value
```

#### Plugin options

Plugin options are defined in YAML under each plugin:

```yaml
plugins:
  - name: gen-client
    options:
      nonNamespaced: true
```

The configuration loader preserves the raw YAML for each plugin's options; it does not validate them. Plugins are constructed once up-front, as soon as code generation starts, and each plugin decodes and validates its own options at that point. Any error surfaces before CRD processing begins rather than partway through rendering.

For plugins that define typed options:

- Unknown fields are rejected
- Field types must match exactly (e.g. `true`, not `"true"`)
- Invalid or unexpected shapes will result in an error when plugins are initialized

For plugins that do not accept options:

- Providing any options will result in an error

#### `get-conditions`

Generates a `GetConditions() []metav1.Condition` method on the root kind type, for use with condition-aware controllers. The CRD's status schema must contain a `conditions` field of the standard `metav1.Condition` array type.

```yaml
plugins:
  - name: get-conditions
```

No options are supported for this plugin.

#### `gen-client`

Adds [`+genclient`](https://github.com/kubernetes/code-generator) markers before the root kind type definition, so that `k8s.io/code-generator`'s `client-gen` tool generates a typed client for the resource.

```yaml
plugins:
  - name: gen-client
```

The following option is supported:

| Option | Type | Default | Description |
|---|---|---|---|
| `nonNamespaced` | bool | `false` | Adds `+genclient:nonNamespaced` for cluster-scoped resources. |

Example for a cluster-scoped resource:

```yaml
plugins:
  - name: gen-client
    options:
      nonNamespaced: true
```

With `nonNamespaced: true` the generated file will contain:

```go
// +genclient
// +genclient:nonNamespaced
// +kubebuilder:object:root=true
type MyResource struct { ... }
```

Without the option:

```go
// +genclient
// +kubebuilder:object:root=true
type MyResource struct { ... }
```

#### `gen-deepcopy`

Emits the `+k8s:deepcopy-gen` markers that controller-gen consumes to produce `DeepCopy` methods. List the plugin to turn marker generation on; omit it to turn off.

```yaml
plugins:
  - name: gen-deepcopy
```

No options. When listed, crd2go adds `+k8s:deepcopy-gen=package` to `doc.go` and `+k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object` before each root kind type.

#### `gen-applyconfiguration`

Emits the `+kubebuilder:ac` markers and the `SchemeGroupVersion` alias that the generated apply-configuration code expects (k8s.io/code-generator convention).

```yaml
plugins:
  - name: gen-applyconfiguration
    options:
      outputPackage: ../../applyconfiguration   # optional
```

| Option | Type | Default | Description |
|---|---|---|---|
| `outputPackage` | string | `""` | Value for the `+kubebuilder:ac:output:package` marker. When empty, the marker is omitted. |

### Type naming and schema evolution

crd2go picks Go type names from CRD shapes and resolves collisions (for example by prepending path segments) so output always type-checks. **Those names are not guaranteed to stay the same when inputs change.** Adding or renaming fields, introducing new kinds, or loading more CRDs into the same pass can create new collisions or change resolution order, so **existing generated type identifiers may rename** even when their underlying schema concept did not change.

Treat regenerated code as **replace the package** (or use your own aliases at stable boundaries) rather than assuming a given nested struct name is a stable public API across generator versions or CRD revisions. If you need evolution-stable names, you must enforce them outside crd2go (for example via **`renames`** / **`imports`**, wrappers in your repo, or a dedicated compatibility layer)—the tool does not implement cross-version name pinning today.

### Running controller-gen

crd2go generates Go types, `doc.go` (with `+k8s:deepcopy-gen` and optional `+kubebuilder:ac` markers), and `groupversion_info.go`. It does **not** invoke `controller-gen` itself. After running crd2go, call `controller-gen` separately to generate deepcopy and (optionally) apply configuration code:

```shell
# Generate deepcopy methods
controller-gen object paths=./path/to/generated/types

# Generate apply configuration structs (if applyConfiguration is enabled)
controller-gen applyconfiguration paths=./path/to/generated/types
```

This separation lets you control `controller-gen` flags such as `headerFile` for license headers. A typical Makefile target might look like:

```makefile
generate:
	go tool crd2go -config crd2go.yaml
	controller-gen object paths=./api/v1 output:object:dir=./api/v1
	controller-gen applyconfiguration paths=./api/v1 output:applyconfiguration:dir=./applyconfiguration
```

### Updating embedded test goldens

Integration tests in `pkg/crd2go` compare generator output to files under `internal/testdata/` (embedded via `//go:embed`). They **only assert**; they do not rewrite goldens. When you change generation behavior, refresh the fixtures manually:

1. **Main v1 package** (`internal/testdata/v1`, exercised by `TestGenerateFromCRDs`):
   ```shell
   go run ./cmd/crd2go -config crd2go.yaml
   controller-gen object paths=./internal/testdata/v1 output:object:dir=./internal/testdata/v1
   ```

2. **Refs sample** (`internal/testdata/refs/v1`, exercised by `TestRefs`): regenerate into that directory using the same input the test uses, then run `controller-gen` on `internal/testdata/refs/v1` if you changed types with deepcopy markers:
   ```shell
   go run ./cmd/crd2go -input ./internal/testdata/samplerefs.yaml -output ./internal/testdata/refs/v1
   controller-gen object paths=./internal/testdata/refs/v1 output:object:dir=./internal/testdata/refs/v1
   ```

   Align flags with the test’s `gotype.Request` (e.g. `TypeDict` / preloads) if your output must match exactly; otherwise adjust the YAML or test expectations as needed.

After updating, run `go test ./pkg/crd2go/...` (or `./...`) to confirm no regressions.

### Developing and repository hygiene

Anything under the module that contains `.go` files is a real package: **`go test ./...` builds it all**, without ignoring paths like `tmp_out`. If you generate into a scratch directory here, that package must look like a normal API tree (including **`controller-gen` after crd2go**); otherwise you’ll get compile errors such as missing `DeepCopyObject` on `runtime.Object` types.

**Practical rule:** put throwaway output **outside** the repo or on a **`gitignore`d** path, or remove the directory before running full-tree tests—one half-generated folder is enough to break CI.

Committed trees such as **`internal/testdata/v1`** are different: they are meant to stay complete and in sync with the tests.

## Naming

crd2go assigns each generated struct a Go name from the CRD shape. Types that are structurally identical share one name; among the candidate spellings, the shortest wins.

If two different shapes would use the same name, the generator prepends pieces of the field path from the nested type up toward the root—one segment per pass—until every name is unique. That rule is deterministic and always produces valid Go, but it does not minimize name length in every case (shared path segments can add extra prefixes before types diverge). User **`renames`** and **`imports`** still override or replace generated names where you need something explicit.
