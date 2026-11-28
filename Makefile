.PHONY: templ lint perf-check perf-baseline deadcode-check deadcode-baseline eval eval-bench

GOLANGCI_LINT_VERSION := 2.13.2

templ:
	go tool templ generate -path ./internal/cloud/dashboard

lint:
	@command -v golangci-lint >/dev/null 2>&1 || { \
		echo "golangci-lint v$(GOLANGCI_LINT_VERSION) is required on PATH."; \
		exit 1; \
	}; \
	version="$$(golangci-lint version --short 2>/dev/null)" || { \
		echo "golangci-lint v$(GOLANGCI_LINT_VERSION) is required, but the executable on PATH does not support 'version --short'."; \
		exit 1; \
	}; \
	if [ "$$version" != "$(GOLANGCI_LINT_VERSION)" ]; then \
		echo "golangci-lint v$(GOLANGCI_LINT_VERSION) is required; found '$$version'."; \
		exit 1; \
	fi; \
	golangci-lint run --new-from-rev=HEAD~

perf-check:
	@if [ -n "$(PERF_RATCHET_AGAINST)" ]; then \
		bash scripts/perf-ratchet.sh --against "$(PERF_RATCHET_AGAINST)"; \
	elif [ "$(PERF_RATCHET_BOOTSTRAP)" = "1" ]; then \
		bash scripts/perf-ratchet.sh --bootstrap; \
	else \
		bash scripts/perf-ratchet.sh; \
	fi

perf-baseline:
	bash scripts/perf-ratchet.sh --update

deadcode-check:
	bash scripts/deadcode-ratchet.sh

deadcode-baseline:
	bash scripts/deadcode-ratchet.sh --update

# Evaluators E1 (context), E2 (search) and E3 (performance) on synthetic
# corpora, built only with -tags eval. Sizes: QBIT_JAY_EVAL_SIZES (default
# 1000,10000; add 100000 to opt in). JSON report: QBIT_JAY_EVAL_JSON=<file>.
eval:
	go test -tags eval -count=1 -timeout 120m ./internal/qbit/eval/... -run TestEval -v

# Store benchmarks over the 10k corpus; QBIT_JAY_EVAL_100K=1 adds 100k.
eval-bench:
	go test -tags eval -run '^$$' -bench . -benchtime=1s -count=5 ./internal/qbit/eval/
