#!/usr/bin/env bash

IMG="hybfkuf/findpvdir:latest"

docker build -t "$IMG" .
docker push "$IMG"
