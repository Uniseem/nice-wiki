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

build test:
	@echo '业务实现未完成：构建与测试入口随对应工作包启用，不报告通过'
	@exit 1
