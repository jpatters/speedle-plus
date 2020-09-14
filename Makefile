gopath := $(shell go env GOPATH)
gitCommit := $(shell git rev-parse --short HEAD)
# go version output is "go version go1.11.2 linux/amd64"
goVersion := $(word 3,$(shell go version))
goLDFlags := -ldflags "-X main.gitCommit=${gitCommit} -X main.productVersion=0.1 -X main.goVersion=${goVersion}"

pmsImageRepo := speedle-pms
pmsImageTag := v0.1
adsImageRepo := speedle-ads
adsImageTag := v0.1

.PHONY: all
all: build install

.PHONY: build
build: buildPms buildAds buildSpctl

.PHONY: buildPms
buildPms:
	go build ${goLDFlags} -o ./.bin/speedle-pms github.com/teramoby/speedle-plus/cmd/speedle-pms

.PHONY: buildAds
buildAds:
	go build ${goLDFlags} -o ./.bin/speedle-ads github.com/teramoby/speedle-plus/cmd/speedle-ads

.PHONY: buildSpctl
buildSpctl:
	go build ${goLDFlags} -o ./.bin/spctl github.com/teramoby/speedle-plus/cmd/spctl

.PHONY: image
image: imagePms imageAds

.PHONY: imagePms
imagePms:
	cp ./.bin/speedle-pms deployment/docker/speedle-pms/.
	docker build -t ${pmsImageRepo}:${pmsImageTag} --rm --no-cache deployment/docker/speedle-pms
	rm deployment/docker/speedle-pms/speedle-pms

.PHONY: imageAds
imageAds:
	cp ./.bin/speedle-ads deployment/docker/speedle-ads/.
	docker build -t ${adsImageRepo}:${adsImageTag} --rm --no-cache deployment/docker/speedle-ads
	rm deployment/docker/speedle-ads/speedle-ads

.PHONY: install
install: installPms installAds installSpctl

.PHONY: installPms
installPms:
	cp ./.bin/speedle-pms ${gopath}/bin/speedle-pms

.PHONY: installAds
installAds:
	cp ./.bin/speedle-ads ${gopath}/bin/speedle-ads

.PHONY: installSpctl
installSpctl:
	cp ./.bin/spctl ${gopath}/bin/spctl

.PHONY: test
test: testAll

.PHONY: testAll
testAll: speedleUnitTests testSpeedleRest testSpeedleGRpc testSpctl testSpeedleRestADSCheck testSpeedleGRpcADSCheck testSpeedleTls

.PHONY: speedleUnitTest
speedleUnitTests:
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/cfg
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/eval
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/store/file
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/store/etcd
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/store/mongodb
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/cmd/spctl/pdl
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/suid
	go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/assertion
	go clean -testcache
	STORE_TYPE=etcd go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/eval
	go clean -testcache
	STORE_TYPE=mongodb go test ${TEST_OPTS} github.com/teramoby/speedle-plus/pkg/eval

.PHONY: testSpeedleRest
testSpeedleRest:
	pkg/svcs/pmsrest/run_file_test.sh
	pkg/svcs/pmsrest/run_etcd_test.sh
	pkg/svcs/pmsrest/run_mongodb_test.sh

.PHONY: testSpeedleGRpc
testSpeedleGRpc:
	pkg/svcs/pmsgrpc/run_file_test.sh
	pkg/svcs/pmsgrpc/run_etcd_test.sh
	pkg/svcs/pmsgrpc/run_mongodb_test.sh

.PHONY: testSpeedleRestADSCheck
testSpeedleRestADSCheck:
	pkg/svcs/adsrest/run_file_test.sh
	pkg/svcs/adsrest/run_etcd_test.sh
	pkg/svcs/adsrest/run_mongodb_test.sh

.PHONY: testSpeedleGRpcADSCheck
testSpeedleGRpcADSCheck:
	pkg/svcs/adsgrpc/run_file_test.sh
	pkg/svcs/adsgrpc/run_etcd_test.sh
	pkg/svcs/adsgrpc/run_mongodb_test.sh

.PHONY: testSpctl
testSpctl:
	cmd/spctl/command/run_file_test.sh
	cmd/spctl/command/run_etcd_test.sh
	cmd/spctl/command/run_mongodb_test.sh

.PHONY: testSpeedleTls
testSpeedleTls:
	pkg/svcs/pmsrest/tls_test.sh
	pkg/svcs/pmsrest/tls_test-force-client-cert.sh

.PHONY: clean
clean:
	rm -rf ${gopath}/pkg/linux_amd64/github.com/teramoby/speedle-plus
	rm -f ${gopath}/bin/speedle-pms
	rm -f ${gopath}/bin/speedle-ads
	rm -f ${gopath}/bin/spctl
	rm -rf ./.bin

GOLANG_DOCKER_MAKE := docker run \
	--rm \
	-it \
	-v $(PWD):/src \
	-w /src \
	-e GOCACHE=/src/.go/cache \
	golang:$(subst go,,$(goVersion)) \
	make

.PHONY: docker-%
docker-%:
	$(GOLANG_DOCKER_MAKE) $*
