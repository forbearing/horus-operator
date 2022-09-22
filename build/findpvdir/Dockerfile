# Build the manager binary
FROM golang:1.18 as builder

WORKDIR /workspace
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

# Copy the go source
COPY findpvdir.go findpvdir.go
## cache deps before building and copying source so that we don't need to re-download as much
## and so that source changes don't invalidate our downloaded layer
#RUN go mod download

# Build
RUN GOOS=linux GOARCH=amd64 go build -o findpvdir findpvdir.go

# Use distroless as minimal base image to package the manager binary
# Refer to https://github.com/GoogleContainerTools/distroless for more details
#FROM gcr.io/distroless/static:nonroot
#USER 65532:65532
#FROM alpine:3.15
FROM debian:stable-slim
WORKDIR /
COPY --from=builder /workspace/findpvdir /usr/local/bin/findpvdir

ENTRYPOINT ["sleep", "infinity"]
