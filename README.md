# aeto - aws-eks-tenant-operator

A Kubernetes "tenant" operator.

## Status

![GitHub](https://img.shields.io/badge/status-alpha-blue?style=for-the-badge)
![GitHub](https://img.shields.io/github/license/kristofferahl/aeto?style=for-the-badge)
![GitHub go.mod Go version](https://img.shields.io/github/go-mod/go-version/kristofferahl/aeto?style=for-the-badge)

## Resources

### Core

- Tenant
- Blueprint
- ResourceTemplate
- ResourceSet

### AWS

- Route53 HostedZone
- ACM Certificate
- ACM CertificateConnector

### Event

- EventStreamChunk

### Sustainability

- SavingsPolicy

## Examples

The `config/samples` and `config/default-resources` contains a working default setup with an example tenant.

## Development

### Pre-requisites

- [Go](https://golang.org/) 1.16 or later
- [operator-sdk](https://sdk.operatorframework.io/) 1.15.0
- [Kubebuilder](https://kubebuilder.io/) 3.2.0
- [AWS](https://aws.amazon.com/) account and credentials
- [Kubernetes](https://kubernetes.io/) cluster

### Getting started

```bash
export AWS_ACCESS_KEY_ID=""
export AWS_SECRET_ACCESS_KEY=""
export AWS_SESSION_TOKEN=""
export AWS_REGION='eu-central-1'
make manifests
make install
make run
```

### Running tests

```bash
make test
```
