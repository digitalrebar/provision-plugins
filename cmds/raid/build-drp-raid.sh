#!/bin/sh
mkdir -p embedded
cd drp-raid && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../embedded/drp-raid.amd64.linux
