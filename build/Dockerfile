# Build the manager binary
FROM golang@sha256:d3f734e1f46ec36da8c1bce67cd48536138085289e24cfc8765f483c401b7d96 as builder

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

FROM clearlinux@sha256:d3dd73575d2eb9c6ffb635c82b266fa9266591db844ac9f41014c0af415992c9
WORKDIR /
COPY --from=builder /workspace/manager .
COPY build/manifests/ /power-manifests/
USER 1001

ENTRYPOINT ["/manager"]
