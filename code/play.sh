#!/bin/bash

docker run -it --rm \
  -p 8000:8000 \
  -v $PWD:/xml \
  -w /xml \
  -e GOPROXY=https://goproxy.io,direct \
  golang:1.16.3-buster
