# virt-template

A KubeVirt add-on providing native, user-friendly templating workflows for
KubeVirt virtual machines.

## Quick Example

```yaml
# Define a template
apiVersion: template.kubevirt.io/v1alpha1
kind: VirtualMachineTemplate
metadata:
  name: fedora
spec:
  parameters:
    - name: NAME
      required: true
    - name: INSTANCETYPE
      value: u1.medium
  virtualMachine:
    metadata:
      name: ${NAME}
    spec:
      instancetype:
        name: u1.medium
      preference:
        name: fedora
    runStrategy: Always
    template:
      spec:
        domain:
          devices: {}
        terminationGracePeriodSeconds: 180
        volumes:
          - containerDisk:
              image: quay.io/containerdisks/fedora:latest
            name: containerdisk-0
```

```sh
# Process and create a VM
virttemplatectl process fedora -p NAME=my-fedora --create
```

## Description

`virt-template` enables users to create, share, and manage VM blueprints
directly within the cluster. The project provides native support for in-cluster
templating through the `VirtualMachineTemplate` custom resource, which allows
you to define reusable VM templates with configurable parameters that can be
substituted at runtime to create VirtualMachine instances.

Developed as a separate operator outside core KubeVirt, `virt-template` is
influenced by OpenShift's Template CRD and designed to provide a more
traditional virtualization experience within Kubernetes.

### Components

- `virt-template-controller`: Kubernetes controller that watches and validates
  VirtualMachineTemplate resources
- `virt-template-apiserver`: API server providing subresource APIs for templates
- `virttemplatectl`: CLI tool for local template processing and management

### Key Features

- **VM Blueprints**: Create reusable VM templates with parameter placeholders
  using `${NAME}` syntax
- **Parameter Substitution**: Define static values, generated values, or
  required parameters for flexible template instantiation
- **Server-Side Processing**: Process templates in-cluster via the `process`
  subresource API
- **Cross-Namespace Sharing**: Share and reuse templates across namespaces
  within your cluster

## Getting Started

### Prerequisites

- Go version v1.24.0+
- Container tool: Podman (default) or Docker
- kubectl

### Development

**Build binaries locally:**

```sh
make build              # Build controller binary
make build-apiserver    # Build apiserver binary
make build-virttemplatectl  # Build CLI tool
```

**Run linting and formatting:**

```sh
make fmt vet lint
```

**Run unit tests:**

```sh
make test
```

### Deployment

#### Quick Start with kubevirtci

The easiest way to develop and test is using kubevirtci:

```sh
make cluster-up        # Start local KubeVirt cluster
make cluster-sync      # Build, push, and deploy to cluster
make cluster-functest  # Run functional tests
make cluster-down      # Stop cluster
```

#### Manual Deployment

**Build and push container images:**

```sh
make container-build container-push \
  IMG_REGISTRY=<registry> \
  IMG_TAG=<tag>
```

The build supports multi-arch images (amd64, arm64, s390x). Use
`CONTAINER_TOOL=docker` or `CONTAINER_TOOL=podman`.

**Install CRDs and deploy controllers:**

```sh
make install
make deploy IMG_REGISTRY=<registry> IMG_TAG=<tag>
```

**Create a sample VirtualMachineTemplate:**

```sh
kubectl apply -f config/samples/template_v1alpha1_virtualmachinetemplate.yaml
```

### Uninstall

```sh
kubectl delete vmt --all  # Delete sample instances
make undeploy             # Remove controllers
make uninstall            # Remove CRDs
```

### Using virttemplatectl

Process a template locally:

```sh
virttemplatectl process -f template.yaml -p NAME=myvm -p INSTANCETYPE=u1.small
```

## Usage

### VirtualMachineTemplate CRD

The `VirtualMachineTemplate` custom resource allows you to define reusable VM
blueprints with parameters:

```yaml
apiVersion: template.kubevirt.io/v1alpha1
kind: VirtualMachineTemplate
metadata:
  name: my-template
spec:
  parameters:
    - name: NAME
      description: Name of the VM
      required: true
    - name: MEMORY
      description: Memory size
      value: "2Gi"
    - name: PASSWORD
      generate: expression
      from: "[a-zA-Z0-9]{16}"
  virtualMachine:
    metadata:
      name: ${NAME}
[...]
```

### Parameter Substitution

Parameters are referenced using `${PARAMETER_NAME}` syntax. They can have:

- **Static values**: Pre-defined default values (`value: "2Gi"`)
- **Generated values**: Random values from expression generator
- **Required flag**: Mark parameters as mandatory (`required: true`)

#### Parameter Generation

The `expression` generator creates random values using regex-like syntax:

```yaml
parameters:
  - name: PASSWORD
    generate: expression
    from: "[a-zA-Z0-9]{16}"  # Generates 16 random alphanumeric chars
  - name: API_KEY
    generate: expression
    from: "[A-F0-9]{32}"      # Generates 32 random hex chars (uppercase)
```

Supported character classes:

- `[a-z]` - lowercase letters
- `[A-Z]` - uppercase letters
- `[0-9]` - digits
- `[a-zA-Z0-9]` - alphanumeric
- `\w` - word characters (letters, digits, underscore)
- `\d` - digits
- `\a` - alphabetic characters
- `\A` - special characters

### Processing Templates

**Process VirtualMachineTemplate via API:**

```sh
virttemplatectl process my-template \
  -p NAME=myvm \
  -p MEMORY=4Gi
```

**Process VirtualMachineTemplate and create VirtualMachine via API:**

```sh
virttemplatectl process my-template \
  -p NAME=myvm \
  -p MEMORY=4Gi \
  --create
```

**Process local template file:**

```sh
virttemplatectl process -f my-template.yaml \
  -p NAME=myvm \
  -p MEMORY=4Gi
```

**Process from stdin:**

```sh
cat my-template.yaml | virttemplatectl process -f - \
  -p NAME=myvm \
  -p MEMORY=4Gi
```

## Distribution

### Build Installer Bundle

Generate a single YAML file with all resources:

```sh
make build-installer IMG_REGISTRY=<registry> IMG_TAG=<tag>
```

This creates `dist/install.yaml` which can be deployed with:

```sh
kubectl apply -f dist/install.yaml
```

## Development Tools

Run `make help` to see all available make targets.

**Kubevirtci workflows:**

The project provides two kubevirtci-based development environments:

- `cluster-*` targets: Test with a stable KubeVirt release
- `kubevirt-*` targets: Test with KubeVirt from git main

Example workflow:

```sh
make cluster-up        # Start kubevirtci cluster
make cluster-sync      # Build, deploy, and test
make cluster-functest  # Run functional tests
make cluster-down      # Stop cluster
```

## Contributing

Contributions are welcome! Please ensure:

- All tests pass: `make test functest`
- `make all` passes (runs formatting, vetting, linting, and code generation)

## License

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

Copyright The KubeVirt Authors.
