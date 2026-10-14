PYTHON ?= python3

.PHONY: check privacy summary docs build gen test

check:
	$(PYTHON) scripts/check_docs.py

privacy:
	$(PYTHON) scripts/check_docs.py --privacy-only

summary:
	$(PYTHON) scripts/check_docs.py --summary

docs:
	$(PYTHON) scripts/render_docs.py

build gen test:
	@echo '当前为设计阶段；业务实现与对应入口尚未建立，不报告构建或测试通过'
	@exit 1
