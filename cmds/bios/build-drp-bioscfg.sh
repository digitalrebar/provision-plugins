#!/bin/sh
mkdir -p embedded
cd bioscfg && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../embedded/drp-bioscfg.amd64.linux
