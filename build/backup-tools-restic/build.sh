#!/usr/bin/env bash

IMG="hybfkuf/backup-tools-restic:latest"

docker build -t "$IMG" .
docker push "$IMG"

#docker run -d --rm hybfkuf/backup-tools:latest sleep infinity
