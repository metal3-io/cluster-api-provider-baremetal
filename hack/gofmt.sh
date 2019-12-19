#!/bin/sh

set -eux

IS_CONTAINER=${IS_CONTAINER:-false}
CONTAINER_RUNTIME=${CONTAINER_RUNTIME:-podman}

if [ "${IS_CONTAINER}" != "false" ]; then
  export XDG_CACHE_HOME="/tmp/.cache"
  mkdir /tmp/unit
  cp -r ./* /tmp/unit
  cd /tmp/unit
  go fmt ./pkg/... ./cmd/...
else
  "${CONTAINER_RUNTIME}" run --rm \
    --env IS_CONTAINER=TRUE \
    --volume "${PWD}:/workdir:ro,z" \
    --entrypoint sh \
    --workdir /workdir \
    registry.hub.docker.com/library/golang:1.12 \
    /workdir/hack/gofmt.sh
fi;
