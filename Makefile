# The repo root is not a Go module — it's a workspace (go.work) over several
# per-demo modules, so `go build ./...` from here has nothing to resolve.
# These targets fan out across the real modules instead.

MODULES := \
	./session-1-fundamentals/otel-quickstart \
	./session-1-fundamentals/seed-sample-service \
	./session-1-fundamentals/instrumentation-traps \
	./session-2-feedback-loops \
	./session-3-business-case \
	./session-4-slis-slos

.PHONY: verify build vet test fmt fmt-check tidy

verify: fmt-check build vet test

build:
	@for m in $(MODULES); do echo "build $$m"; (cd $$m && go build ./...) || exit 1; done

vet:
	@for m in $(MODULES); do echo "vet   $$m"; (cd $$m && go vet ./...) || exit 1; done

test:
	@for m in $(MODULES); do echo "test  $$m"; (cd $$m && go test ./...) || exit 1; done

fmt:
	@gofmt -w session-1-fundamentals session-2-feedback-loops session-3-business-case session-4-slis-slos

fmt-check:
	@out=$$(gofmt -l session-1-fundamentals session-2-feedback-loops session-3-business-case session-4-slis-slos); \
	if [ -n "$$out" ]; then echo "gofmt needed:"; echo "$$out"; exit 1; fi; \
	echo "gofmt clean"

tidy:
	@for m in $(MODULES); do echo "tidy  $$m"; (cd $$m && go mod tidy) || exit 1; done

.PHONY: vulncheck tidy-check

vulncheck:
	@for m in $(MODULES); do \
		echo "vuln  $$m"; \
		(cd $$m && go run golang.org/x/vuln/cmd/govulncheck@latest ./...) || exit 1; \
	done

# Fails if go.mod/go.sum are not what `go mod tidy` would produce. Catches
# dependency drift, including an indirect dep quietly resolving back down to a
# vulnerable version in the module graph.
tidy-check:
	@for m in $(MODULES); do \
		echo "tidy? $$m"; \
		cp $$m/go.mod /tmp/go.mod.bak && cp $$m/go.sum /tmp/go.sum.bak; \
		(cd $$m && GOWORK=off go mod tidy) || exit 1; \
		if ! diff -q /tmp/go.mod.bak $$m/go.mod >/dev/null || ! diff -q /tmp/go.sum.bak $$m/go.sum >/dev/null; then \
			echo "  $$m: go.mod/go.sum are not tidy; run 'make tidy' and commit the result"; \
			cp /tmp/go.mod.bak $$m/go.mod && cp /tmp/go.sum.bak $$m/go.sum; \
			exit 1; \
		fi; \
	done
