#!/bin/bash

PROJECT_DIR="$(cd "`dirname $0`"/..; pwd)"
cd $PROJECT_DIR

print_usage() {
  echo "usage: $0 [tty]"
}

if [[ ( $1 == "help" ) || ( $1 == "-h" )  ]]; then
  print_usage
  exit 0
fi

GIT_COMMIT_SHORT=`git rev-parse HEAD | head -c 7`
TTY_FLAG=""

if [[ $1 == "tty" ]]; then
  TTY_FLAG="t"
fi

DOCKER_RUN_OPTS="-i${TTY_FLAG} --rm"
IMAGE_TAG="sidecar-proxy:${GIT_COMMIT_SHORT}"

docker build . -f docker/Dockerfile.test -t $IMAGE_TAG

docker run $DOCKER_RUN_OPTS $IMAGE_TAG

test_result=$?
echo $test_result
echo "^^ exit code"

docker rmi $IMAGE_TAG 

exit $test_result
