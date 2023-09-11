# Current Operator version
VERSION ?= 0.0.1
# Default bundle image tag
BUNDLE_IMG ?= controller-bundle:$(VERSION)
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)

# Image URL to use all building/pushing image targets
IMG ?= controller:latest
# Produce CRDs that work back to Kubernetes 1.11 (no version conversion)
CRD_OPTIONS ?= "crd:crdVersions=v1"

# Get the currently used golang install path (in GOPATH/bin, unless GOBIN is set)
ifeq (,$(shell go env GOBIN))
GOBIN=$(shell go env GOPATH)/bin
else
GOBIN=$(shell go env GOBIN)
endif

all: manifests generate install

# Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate fmt vet manifests
	go test -v ./... -coverprofile cover.out

# Build manager binary
build: generate manifests install
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/manager build/manager/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/nodeagent build/nodeagent/main.go

# Build the Manager and Node Agent images
images: generate manifests install
	docker build -f build/Dockerfile -t intel/power-operator:v2.3.1 .
	docker build -f build/Dockerfile.nodeagent -t intel/power-node-agent:v2.3.1 .

# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

helm-install: generate manifests install
	helm install kubernetes-power-manager-v2.3.1 ./helm/kubernetes-power-manager-v2.3.1

helm-uninstall:
	helm uninstall kubernetes-power-manager-v2.3.1

helm-install-v2.3.0: generate manifests install
	helm install kubernetes-power-manager-v2.3.0 ./helm/kubernetes-power-manager-v2.3.0

helm-uninstall-v2.3.0:
	helm uninstall kubernetes-power-manager-v2.3.0

helm-install-v2.2.0: generate manifests install
	helm install kubernetes-power-manager-v2.2.0 ./helm/kubernetes-power-manager-v2.2.0

helm-uninstall-v2.2.0:
	helm uninstall kubernetes-power-manager-v2.2.0

helm-install-v2.1.0: generate manifests install
	helm install kubernetes-power-manager-v2.1.0 ./helm/kubernetes-power-manager-v2.1.0

helm-uninstall-v2.1.0:
	helm uninstall kubernetes-power-manager-v2.1.0

helm-install-v2.0.0: generate manifests install
	helm install kubernetes-power-manager-v2.0.0 ./helm/kubernetes-power-manager-v2.0.0

helm-uninstall-v2.0.0:
	helm uninstall kubernetes-power-manager-v2.0.0

# Install CRDs into a cluster
install: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl apply -f -

# Uninstall CRDs from a cluster
uninstall: manifests kustomize
	$(KUSTOMIZE) build config/crd | kubectl delete -f -

# Deploy controller in the configured Kubernetes cluster in ~/.kube/config
deploy: manifests kustomize
	cd config/manager && $(KUSTOMIZE) edit set image controller=${IMG}
	$(KUSTOMIZE) build config/default | kubectl apply -f -

# Generate manifests e.g. CRD, RBAC etc.
manifests: controller-gen
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet -composites=false ./...

# Testing the generation of TLS certificates
tls:
	./build/gen_test_certs.sh

# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the Manager's image
build-controller:
	docker build -f build/Dockerfile -t intel-power-operator .

# Build the Node Agent's image
build-agent:
	docker build -f build/Dockerfile.nodeagent -t intel-power-node-agent .

# Push the docker image
docker-push:
	docker push ${IMG}

# find or download controller-gen
# download controller-gen if necessary
controller-gen:
ifeq (, $(shell which controller-gen))
	go install sigs.k8s.io/controller-tools/cmd/controller-gen@v0.9.0
CONTROLLER_GEN=$(GOBIN)/controller-gen
else
CONTROLLER_GEN=$(shell which controller-gen)
endif

kustomize:
ifeq (, $(shell which kustomize))
	go install sigs.k8s.io/kustomize/kustomize/v4@latest
KUSTOMIZE=$(GOBIN)/kustomize
else
KUSTOMIZE=$(shell which kustomize)
endif

# Generate bundle manifests and metadata, then validate generated files.
.PHONY: bundle
bundle: manifests
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS)
	operator-sdk bundle validate ./bundle

# Build the bundle image.
.PHONY: bundle-build
bundle-build:
	docker build -f bundle.Dockerfile -t $(BUNDLE_IMG) .

coverage:
	go test -v -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Average code coverage: $$(go tool cover -func coverage.out | awk 'END {print $$3}' | sed 's/\..*//')%" 
	@if [ $$(go tool cover -func coverage.out | awk 'END {print $$3}' | sed 's/\..*//') -lt 85 ]; then \
		echo "WARNING: Total unit test coverage below 85%"; false; \
	fi
