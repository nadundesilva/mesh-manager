# Mesh Manager

<img src="./images/mesh-manager-logo-transparent.png" alt="Mesh Manager Logo" width="300"/>

[![Main Branch Build](https://github.com/nadundesilva/mesh-manager/actions/workflows/branch-build.yaml/badge.svg)](https://github.com/nadundesilva/mesh-manager/actions/workflows/branch-build.yaml)
[![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

[![Release](https://img.shields.io/github/release/nadundesilva/mesh-manager.svg?style=flat-square)](https://github.com/nadundesilva/mesh-manager/releases/latest)
[![Docker Image](https://img.shields.io/docker/image-size/nadunrds/mesh-manager/latest?style=flat-square)](https://hub.docker.com/r/nadunrds/mesh-manager)
[![Docker Pulls](https://img.shields.io/docker/pulls/nadunrds/mesh-manager?style=flat-square)](https://hub.docker.com/r/nadunrds/mesh-manager)

Mesh Manager is Kubernetes Operator for managing microservices at scale. One of the main problems that any system faces with scale
is managing microservices and ensuring that the system runs properly. Mesh Manager aims to address this issue.

## Features

- Enforcing strictly specifying dependencies
- Prevention of missing dependency issues

## How to Use

### Prerequisites

The following tools are expected to be installed and ready.

- Kubectl
- Operator SDK

The following tools can be either installed on your own or let the installation scripts handle it.

- OLM to be installed in the cluster
  OLM can be installed using the [operator-sdk](https://sdk.operatorframework.io/docs/installation/)
  ```bash
  operator-sdk olm install
  ```

### How to Setup Operator

#### Quickstart

Run the following command to apply the controller to your cluster. The `<VERSION>` should be replaced with the release version
to be used (eg:- `0.1.0`) and kubectl CLI should be configured pointing to the cluster in which the controller needs to be started.

```bash
curl -L https://raw.githubusercontent.com/nadundesilva/mesh-manager/main/installers/install.sh | bash -s <VERSION>
```

#### Manual Installation

- Make sure all the pre-requisites are installed (including the dependencies which are normally installed by the installation scripts)
- Install the Operator Bundle using the Operator SDK. The `<VERSION>` should be replaced with the release version
  to be used (eg:- `0.1.0`) and kubectl CLI should be configured pointing to the cluster in which the controller needs to be started.
  ```bash
  operator-sdk run bundle docker.io/nadunrds/mesh-manager-bundle:<VERSION>
  ```

### Examples

Examples for the CRDs used by the Operator can be found in the [samples](./config/samples) directory.

### How to Cleanup Operator

#### Quick Remove

Run the following command to remove the controller from your cluster. Kubectl CLI should be configured pointing to the cluster in which the controller needs to be started.

```bash
curl -L https://raw.githubusercontent.com/nadundesilva/mesh-manager/main/installers/uninstall.sh | bash -s
```

#### Manual Removal

Remove the controller from your cluster by running the following command.

```bash
operator-sdk cleanup mesh-manager
```

## Support

:grey_question: If you need support or have a question about the Mesh Manager, reach out through [Discussions](https://github.com/nadundesilva/mesh-manager/discussions).

:bug: If you have found a bug and would like to get it fixed, try opening a [Bug Report](https://github.com/nadundesilva/mesh-manager/issues/new?labels=Type%2FBug&template=bug-report.md).

:bulb: If you have a new idea or want to get a new feature or improvement added to the Mesh Manager, try creating a [Feature Request](https://github.com/nadundesilva/mesh-manager/issues/new?labels=Type%2FFeature&template=feature-request.md).

## Development

Please refer to the [Development Guide](./DEVELOPMENT.md)](./DEVELOPMENT.md) for more information. Contributions are welcome.
