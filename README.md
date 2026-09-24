# Kubernetes Zones Operator

[![Release](https://github.com/ZEISS/zones-operator/actions/workflows/release.yml/badge.svg)](https://github.com/ZEISS/zones-operator/actions/workflows/release.yml)
[![Taylor Swift](https://img.shields.io/badge/secured%20by-taylor%20swift-brightgreen.svg)](https://twitter.com/SwiftOnSecurity)
[![Volkswagen](https://auchenberg.github.io/volkswagen/volkswargen_ci.svg?v=1)](https://github.com/auchenberg/volkswagen)
[![License](https://img.shields.io/badge/License-Apache%202.0-blue.svg)](https://opensource.org/licenses/Apache-2.0)

A Kubernetes operator for creating [vCluster](https://vcluster.com/) zones.

## Installation

[Helm](https://helm.sh/) can be used to install the `zones-operator` to your Kubernetes cluster.

```shell
helm repo add zones-operator https://zeiss.github.io/zones-operator
helm repo update
helm search repo zones-operator
```

## License

[Apache 2.0](/LICENSE)