#!/bin/bash -x
mkdir -p $GOPATH/bin
curl -L https://github.com/kubernetes-sigs/kustomize/releases/download/v3.2.0/kustomize_3.2.0_linux_amd64 -o $GOPATH/bin/kustomize
chmod +x $GOPATH/bin/kustomize
