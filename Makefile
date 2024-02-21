export APP_NAME=intel-kubernetes-power-manager
# Current Operator version
VERSION ?= 2.4.0
# parameter used for helm chart image
HELM_CHART ?= v2.4.0
HELM_VERSION := $(shell echo $(HELM_CHART) | cut -d "v" -f2)
# used to detemine if certain targets should build for openshift
OCP ?= false
# Default bundle image tag
BUNDLE_IMG ?= intel-kubernetes-power-manager-bundle:$(VERSION)
# version of ocp being supported
OCP_VERSION=4.13
# image used for building the dockerfile for ocp
OCP_IMAGE=redhat/ubi9-minimal@sha256:06d06f15f7b641a78f2512c8817cbecaa1bf549488e273f5ac27ff1654ed33f0
# Options for 'bundle-build'
ifneq ($(origin CHANNELS), undefined)
BUNDLE_CHANNELS := --channels=$(CHANNELS)
endif
ifneq ($(origin DEFAULT_CHANNEL), undefined)
BUNDLE_DEFAULT_CHANNEL := --default-channel=$(DEFAULT_CHANNEL)
endif
BUNDLE_METADATA_OPTS ?= $(BUNDLE_CHANNELS) $(BUNDLE_DEFAULT_CHANNEL)
IMGTOOL ?= docker
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

ifneq (, $(IMAGE_REGISTRY))
IMAGE_TAG_BASE = $(IMAGE_REGISTRY)/$(APP_NAME)
else
IMAGE_TAG_BASE = $(APP_NAME)
endif

TLS_VERIFY ?= false

# The image tag given to the resulting catalog image (e.g. make catalog-build CATALOG_IMG=example.com/operator-catalog:v0.2.0).
CATALOG_IMG ?= $(IMAGE_TAG_BASE)-catalog:v$(VERSION)

# Set CATALOG_BASE_IMG to an existing catalog image tag to add $BUNDLE_IMGS to that image.
ifneq ($(origin CATALOG_BASE_IMG), undefined)
FROM_INDEX_OPT := --from-index $(CATALOG_BASE_IMG)
endif

BUNDLE_IMGS ?= $(BUNDLE_IMG)

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

.PHONY: all test build images images-ocp run

all: manifests generate install

# Run tests
ENVTEST_ASSETS_DIR = $(shell pwd)/testbin
test: generate fmt vet manifests
	go test -v ./... -coverprofile cover.out

# Build manager binary
build: generate manifests install
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/manager build/manager/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/nodeagent build/nodeagent/main.go

verify-build: gofmt test race coverage tidy clean verify-test
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/manager build/manager/main.go
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO111MODULE=on go build -a -o build/bin/nodeagent build/nodeagent/main.go	

# Build the Manager and Node Agent images
images: generate manifests install
	 $(IMGTOOL) build -f build/Dockerfile -t intel/power-operator:v$(VERSION) .
	 $(IMGTOOL) build -f build/Dockerfile.nodeagent -t intel/power-node-agent:v$(VERSION) .

images-ocp: generate manifests install
	 $(IMGTOOL) build --build-arg="BASE_IMAGE=$(OCP_IMAGE)" --build-arg="MANIFEST=build/manifests/ocp/power-node-agent-ds.yaml" -f build/Dockerfile -t intel/power-operator_ocp-$(OCP_VERSION):v$(VERSION) .
	 $(IMGTOOL) build --build-arg="BASE_IMAGE=$(OCP_IMAGE)" -f build/Dockerfile.nodeagent -t intel/power-node-agent_ocp-$(OCP_VERSION):v$(VERSION) .
# Run against the configured Kubernetes cluster in ~/.kube/config
run: generate fmt vet manifests
	go run ./main.go

.PHONY: helm-install helm-uninstall 
helm-install:
ifeq (true, $(OCP))
	$(eval HELM_FLAG:=--set ocp=true)
	$(eval OCP_SUFFIX:=_ocp-$(OCP_VERSION))
endif
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/kubernetes-power-manager/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/kubernetes-power-manager/Chart.yaml
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/crds/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/crds/Chart.yaml 
	helm install kubernetes-power-manager-crds ./helm/crds
	helm dependency update ./helm/kubernetes-power-manager
	helm install kubernetes-power-manager-$(HELM_CHART) ./helm/kubernetes-power-manager --set operator.container.image=intel/power-operator$(OCP_SUFFIX):$(HELM_CHART) $(HELM_FLAG)

helm-uninstall:
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/kubernetes-power-manager/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/kubernetes-power-manager/Chart.yaml 
	sed -i 's/^version:.*$$/version: $(HELM_VERSION)/' helm/crds/Chart.yaml 
	sed -i 's/^appVersion:.*$$/appVersion: \"$(HELM_CHART)\"/' helm/crds/Chart.yaml 
	helm uninstall kubernetes-power-manager-$(HELM_CHART)
	helm uninstall kubernetes-power-manager-crds

.PHONY: install uninstall deploy manifests fmt vet tls
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
ifeq (false, $(OCP))
	sed -i 's/- .*\/rbac\.yaml/- \.\/rbac.yaml/' config/rbac/kustomization.yaml
	sed -i 's/- .*\/role\.yaml/- \.\/role.yaml/' config/rbac/kustomization.yaml
else
	sed -i 's/- .*\/rbac\.yaml/- \.\/ocp\/rbac.yaml/' config/rbac/kustomization.yaml
	sed -i 's/- .*\/role\.yaml/- \.\/ocp\/role.yaml/' config/rbac/kustomization.yaml
endif
	$(CONTROLLER_GEN) $(CRD_OPTIONS) rbac:roleName=manager-role webhook crd paths="./..." output:crd:artifacts:config=config/crd/bases

# Run go fmt against code
fmt:
	go fmt ./...

# Run go vet against code
vet:
	go vet -composites=false ./...

# Testing the generation of TLS certificates
tls:
	./build/gen_test_certs.sh

.PHONY: generate build-controller build-agent build-controller-ocp build-agent-ocp
# Generate code
generate: controller-gen
	$(CONTROLLER_GEN) object:headerFile="hack/boilerplate.go.txt" paths="./..."

# Build the Manager's image
build-controller:
	$(IMGTOOL) build -f build/Dockerfile -t intel-power-operator .

# Build the Node Agent's image
build-agent:
	$(IMGTOOL) build -f build/Dockerfile.nodeagent -t intel-power-node-agent .

build-controller-ocp:
	$(IMGTOOL) build --build-arg="BASE_IMAGE=$(OCP_IMAGE)" -f build/Dockerfile -t intel/power-operator_ocp-$(OCP_VERSION):v$(VERSION) .

build-agent-ocp:
	$(IMGTOOL) build --build-arg="BASE_IMAGE=$(OCP_IMAGE)" -f build/Dockerfile.nodeagent -t intel/power-node-agent_ocp-$(OCP_VERSION):v$(VERSION) .

.PHONY: docker-push controller-gen kustomize bundle bundle-build bundle-push
# Push the image
push:
	$(IMGTOOL) push ${IMG}

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
bundle: update manifests kustomize
# directory used to get image name for bundle
ifeq (false, $(OCP))
	sed -i 's/\.\.\/manager.*$$/\.\.\/manager/' config/default/kustomization.yaml
else
	sed -i 's/\.\.\/manager.*$$/\.\.\/manager\/ocp/' config/default/kustomization.yaml
	sed -i 's/- .*rbac\.yaml/- \.\/ocp\/rbac.yaml/' config/rbac/kustomization.yaml
	sed -i 's/- .*role\.yaml/- \.\/ocp\/role.yaml/' config/rbac/kustomization.yaml
endif
	operator-sdk generate kustomize manifests -q
	cd config/manager && $(KUSTOMIZE) edit set image controller=$(IMG)
	$(KUSTOMIZE) build config/manifests | operator-sdk generate bundle -q --use-image-digests --overwrite --version $(VERSION) $(BUNDLE_METADATA_OPTS) 
	operator-sdk bundle validate ./bundle

# Build the bundle image.
bundle-build:
	$(IMGTOOL) build -f bundle.Dockerfile -t $(BUNDLE_IMG) .
# Push bundle image
bundle-push:
	$(IMGTOOL) push $(BUNDLE_IMG)
.PHONY: opm catalog-build catalog-push coverage update
OPM_VERSION = v1.26.2
OPM = ./bin/opm
opm: ## Download opm locally if necessary.
ifeq (,$(wildcard $(OPM)))
ifeq (,$(shell which opm 2>/dev/null))
	@{ \
	set -e ;\
	mkdir -p $(dir $(OPM)) ;\
	OS=$(shell go env GOOS) && ARCH=$(shell go env GOARCH) && \
	curl -sSLo $(OPM) https://github.com/operator-framework/operator-registry/releases/download/${OPM_VERSION}/$${OS}-$${ARCH}-opm ;\
	chmod +x $(OPM) ;\
	}
else
OPM = $(shell which opm)
endif
endif

.PHONY: catalog-build
catalog-build: opm ## Build a catalog image.
	$(OPM) index add --container-tool $(IMGTOOL) --mode semver --tag $(CATALOG_IMG) --bundles $(BUNDLE_IMGS) $(if ifeq $(TLS_VERIFY) false, --skip-tls) $(FROM_INDEX_OPT)

# Push the catalog image.
.PHONY: catalog-push
catalog-push: ## Push a catalog image.
	 $(IMGTOOL) push ${CATALOG_IMG}

coverage:
	go test -v -coverprofile=coverage.out ./controllers/ ./pkg/...
	go tool cover -html=coverage.out -o coverage.html
	@echo "Average code coverage: $$(go tool cover -func coverage.out | awk 'END {print $$3}' | sed 's/\..*//')%" 
	@if [ $$(go tool cover -func coverage.out | awk 'END {print $$3}' | sed 's/\..*//') -lt 85 ]; then \
                echo "Total unit test coverage below 85%"; false; \
        fi

tidy:
	go mod tidy

verify-test: tidy
	go test -count=1 -v ./...

race: tidy
        CGO_ENABLED=1 go test -count=1 -race -v ./...

clean:
	go clean --cache
	rm -r build/bin/manager
	rm -r build/bin/nodeagent

gofmt:
	gofmt -w .

update:
	sed -i 's/intel\/power-operator.*$$/intel\/power-operator:v$(VERSION)/' config/manager/manager.yaml
	sed -i 's/intel\/power-operator.*$$/intel\/power-operator_ocp-$(OCP_VERSION):v$(VERSION)/' config/manager/ocp/manager.yaml
	sed -i 's/intel\/power-node-agent.*$$/intel\/power-node-agent:v$(VERSION)/' build/manifests/power-node-agent-ds.yaml
	sed -i 's/intel\/power-node-agent.*$$/intel\/power-node-agent_ocp-$(OCP_VERSION):v$(VERSION)/' build/manifests/ocp/power-node-agent-ds.yaml