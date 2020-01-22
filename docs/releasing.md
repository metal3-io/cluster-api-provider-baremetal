# Releasing
<!-- START doctoc generated TOC please keep comment here to allow auto update -->
<!-- DON'T EDIT THIS SECTION, INSTEAD RE-RUN doctoc TO UPDATE -->


- [Prerequisites](#prerequisites)
  - [`gcloud`](#gcloud)
  - [`docker`](#docker)
- [Output](#output)
  - [Expected artifacts](#expected-artifacts)
  - [Artifact locations](#artifact-locations)
- [Process](#process)
  - [Permissions](#permissions)

<!-- END doctoc generated TOC please keep comment here to allow auto update -->

## Prerequisites


### `docker`

You must have push access `quay.io/metal3-io`. 

## Output

### Expected artifacts

1. A container image of the shared cluster-api-provider-baremetal manager
2. A git tag 
3. Manifest file - `infrastructure-components.yaml`

### Artifact locations

1. The container image is found in the registry `quay.io/metal3-io` with an image
   name of `cluster-api-provider-baremetal-<ARCH>` and a tag that matches the release version. 

## Process

For version v0.x.y:

1. Create an annotated tag `git tag -a v0.x.y -m v0.x.y`
    1. To use your GPG signature when pushing the tag, use `git tag -s [...]` instead
1. Push the tag to the GitHub repository `git push origin v0.x.y`
    1. NB: `origin` should be the name of the remote pointing to `github.com/metal3-io/cluster-api-provider-baremetal`
1. Run `make release` to build artifacts 
1. [Create a release in GitHub](https://help.github.com/en/github/administering-a-repository/creating-releases) based on the tag created above.

### Permissions

Releasing requires a particular set of permissions.

* Tag push access to the GitHub repository
* GitHub Release creation access
