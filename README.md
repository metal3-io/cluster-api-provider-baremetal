# DEPRECATED Cluster API Provider for Managed Bare Metal Hardware

**This repository has been deprecated! Please use
[cluster-api-provider-metal3](https://github.com/metal3-io/cluster-api-provider-metal3)
Do not open pull requests or issues in this repository anymore. The open issues
will remain open as long as needed but new issues should be opened in the
new repository.**

This repository contains a Machine actuator implementation for the
Kubernetes [Cluster API](https://github.com/kubernetes-sigs/cluster-api/).

For more information about this actuator and related repositories, see
[metal3.io](http://metal3.io/).

## Development Environment

* See [metal3-dev-env](https://github.com/metal3-io/metal3-dev-env) for an
  end-to-end development and test environment for
  `cluster-api-provider-baremetal` and
  [baremetal-operator](https://github.com/metal3-io/baremetal-operator).
* [Setting up for tests](docs/dev-setup.md)

## API

See the [API Documentation](docs/api.md) for details about the objects used with
this `cluster-api` provider. You can also see the [cluster deployment
workflow](docs/deployment_workflow.md) for the outline of the
deployment process.

## Architecture

The architecture with the components involved is documented [here](docs/architecture.md)

## Deployment and examples

### Deploy Bare Metal Operator CRDs and CRs

for testing purposes only, when Bare Metal Operator is not deployed

```sh
    make deploy-bmo-cr
```

### Deploy CAPM3

Deploys CAPM3 CRDs and deploys CAPI, CABPK, CACPK and CAPM3 controllers

```sh
    make deploy
```

### Run locally

Runs CAPM3 controller locally

```sh
    kubectl scale -n capm3-system deployment.v1.apps/capm3-controller-manager \
      --replicas 0
    make run
```

### Deploy an example cluster

```sh
    make deploy-examples
```

### Delete the example cluster

```sh
    make delete-examples
```
