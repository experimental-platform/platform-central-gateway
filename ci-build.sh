#!/bin/bash
# THIS ONLY WORK IN OUR CI!

docker run --rm -v /data/jenkins/jobs/${JOB_NAME}/workspace:/usr/src/central-gateway -w /usr/src/central-gateway golang:1.4 /bin/bash -c 'go get -t -d && go test && go build -v'
