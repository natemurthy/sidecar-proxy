.PHONY: docker-build
docker-build:
	rm sidecar-proxy || true
	./docker/build_artifact.sh

.PHONY: docker-test
docker-test:
	./docker/run_unit_tests.sh

.PHONY: build
build:
	rm sidecar-proxy || true
	go build -o sidecar-proxy

.PHONY: mocks
mocks:
	go generate ./...

TEST_COMMAND := $(shell type -p richgo || echo go)
.PHONY: test
test:
	$(TEST_COMMAND) test ./...

.PHONY: coverage-summary
coverage-summary:
	type -p go-carpet > /dev/null || go get -v github.com/msoap/go-carpet
	go-carpet -summary

