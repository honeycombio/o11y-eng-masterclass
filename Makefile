# The repo root is not a Go module — it's a workspace (go.work) over several
# per-demo modules, so `go build ./...` from here has nothing to resolve.
# These targets fan out across the real modules instead.

MODULES := \
	./session-1-fundamentals/otel-quickstart \
	./session-1-fundamentals/seed-sample-service \
	./session-1-fundamentals/instrumentation-traps \
	./session-2-feedback-loops

.PHONY: verify build vet test fmt fmt-check tidy

verify: fmt-check build vet test

build:
	@for m in $(MODULES); do echo "build $$m"; (cd $$m && go build ./...) || exit 1; done

vet:
	@for m in $(MODULES); do echo "vet   $$m"; (cd $$m && go vet ./...) || exit 1; done

test:
	@for m in $(MODULES); do echo "test  $$m"; (cd $$m && go test ./...) || exit 1; done

fmt:
	@gofmt -w session-1-fundamentals session-2-feedback-loops

fmt-check:
	@out=$$(gofmt -l session-1-fundamentals session-2-feedback-loops); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi; \
	echo "gofmt clean"

tidy:
	@for m in $(MODULES); do echo "tidy  $$m"; (cd $$m && go mod tidy) || exit 1; done
