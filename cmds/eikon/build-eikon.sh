#!/bin/bash
mkdir -p embedded

files=(parted-3.2-35.el7.centos.x86_64.rpm)
for i in "${files[@]}"
do
    if [[ ! -e embedded/$i ]] ; then
        curl -o embedded/$i https://s3-us-west-2.amazonaws.com/rackn-sledgehammer/$i
    fi
done

cd eikon-src && GOOS=linux GOARCH=amd64 go build -ldflags "-s -w" -o ../embedded/eikon.amd64.linux
