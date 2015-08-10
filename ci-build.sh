#!/bin/bash
set -e

SRC_PATH=$(pwd)

docker run -v ${SRC_PATH}:/usr/src/central-gateway -w /usr/src/central-gateway golang:1.4 /bin/bash -c 'go get -t -d && go test && go build -v'
