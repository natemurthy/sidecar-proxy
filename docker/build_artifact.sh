#!/bin/bash

PROJECT_DIR="$(cd "`dirname $0`"/..; pwd)"
cd $PROJECT_DIR

SEMVER="0.1.0"

GIT_COMMIT_SHORT=`git rev-parse HEAD | head -c 7`
VERSION_TAG=""
if [[ -z $BUILD_NUMBER ]]; then
  VERSION_TAG="${SEMVER}.${GIT_COMMIT_SHORT}"
else
  VERSION_TAG="${SEMVER}.${GIT_COMMIT_SHORT}.${BUILD_NUMBER}"
fi

TMP_IMAGE_TAG="sidecar-proxy:tmp-${GIT_COMMIT_SHORT}"

rm -f $PROJECT_DIR/sidecar-proxy

docker build . -f docker/Dockerfile.build -t $TMP_IMAGE_TAG
exit_code=$?

if [[ $exit_code == 0 ]]; then
  docker run --name tmp-$GIT_COMMIT_SHORT $TMP_IMAGE_TAG
  docker cp tmp-$GIT_COMMIT_SHORT:/tmp/sidecar-proxy .
  docker build . -f docker/Dockerfile -t sidecar-proxy:$VERSION_TAG
  docker rm tmp-$GIT_COMMIT_SHORT
  docker rmi $TMP_IMAGE_TAG
else
  exit $exit_code
fi

echo "${VERSION_TAG}" > version
