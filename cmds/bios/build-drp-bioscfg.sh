#!/bin/sh
mkdir -p embedded
cd bioscfg && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../embedded/drp-bioscfg.amd64.linux
GOOS=linux GOARCH=ppc64le go build -ldflags "-s -w" -o ../embedded/drp-bioscfg.ppc64le.linux
