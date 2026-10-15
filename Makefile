PYTHON ?= python3

.PHONY: check privacy summary docs gen build test

check:
	$(PYTHON) scripts/check_docs.py

privacy:
	$(PYTHON) scripts/check_docs.py --privacy-only

summary:
	$(PYTHON) scripts/check_docs.py --summary

docs:
	$(PYTHON) scripts/render_docs.py

gen:
	$(PYTHON) scripts/gen_code.py

GO_MODULES := libs/platform $(patsubst %/go.mod,%,$(wildcard modules/*/go.mod))

# 编译门槛（AGENTS“检查与记录”）：Go 代码记 implemented/verified 前必须通过。
build:
	@command -v go >/dev/null || { echo '未找到 go：把 Go 工具链目录加入 PATH（macOS Homebrew 默认 /opt/homebrew/bin）后重试'; exit 1; }
	@for m in $(GO_MODULES); do \
		(cd $$m && GOFLAGS=-mod=readonly go build ./... && GOFLAGS=-mod=readonly go vet ./... \
			&& out=$$(gofmt -l .) && test -z "$$out") || { echo "构建检查失败：$$m $$out"; exit 1; }; \
	done
	@echo 'Go 模块 build/vet/gofmt 通过；不代表业务验收'

test:
	@echo '未登记测试套件：ADR0006 默认不写测试；验证用 make build 与 make check'
