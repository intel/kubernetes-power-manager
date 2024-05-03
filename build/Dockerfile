ARG BASE_IMAGE=scratch
# Build the manager binary
FROM golang@sha256:d5302d40dc5fbbf38ec472d1848a9d2391a13f93293a6a5b0b87c99dc0eaa6ae as builder

WORKDIR /workspace
# Copy the Go Modules manifests
COPY go.mod go.mod
COPY go.sum go.sum
# cache deps before building and copying source so that we don't need to re-download as much
# and so that source changes don't invalidate our downloaded layer
RUN go mod download

# Copy the go source
COPY build/manager/main.go main.go
COPY api/ api/
COPY controllers/ controllers/
COPY pkg/ pkg/

# Build
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o manager main.go

FROM ${BASE_IMAGE}
ARG MANIFEST=build/manifests/power-node-agent-ds.yaml
WORKDIR /
COPY LICENSE /licenses/LICENSE
COPY --from=builder /workspace/manager .
COPY ${MANIFEST} /power-manifests/
USER 10001

ENTRYPOINT ["/manager"]
