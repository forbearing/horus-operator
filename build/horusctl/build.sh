#!/usr/bin/env bash

set -e

IMG="hybfkuf/horusctl:latest"

GOOS=linux GOARCH=amd64 go build -o ./tmp/horusctl horusctl.go
docker build -t "$IMG" .
docker push "$IMG"
