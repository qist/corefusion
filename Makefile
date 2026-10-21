# CoreFusion 构建与运行
#
# 版本号来源：internal/constants/version（可用 make VERSION=xxx 覆盖注入值）
# 前端产物先构建到 web/admin/dist，再复制到 internal/app/embed/dist 供 go:embed 打包。

SHELL := /bin/bash

GO       ?= go
NPM      ?= npm
BINARY   ?= corefusion
VERSION  ?= $(shell cat internal/constants/version 2>/dev/null || echo dev)

# 静态链接：项目使用纯 Go SQLite 驱动，无需 CGO
CGO_ENABLED ?= 0
export CGO_ENABLED

# 默认产出版本：-trimpath 去掉本机路径、-s -w 去掉符号表与 DWARF、-buildid= 禁buildid；
# 调试时用 make GOBUILDFLAGS= LDFLAGS="-X corefusion/internal/constants.Version=$(VERSION)" 覆盖。
GOBUILDFLAGS ?= -trimpath
LDFLAGS      ?= -s -w -buildid= -X corefusion/internal/constants.Version=$(VERSION)

FRONTEND_DIR := web/admin
EMBED_DIR    := internal/app/embed/dist
AGENT_DIR    := data/binaries/active/corefusion-agent
DIST_DIR     := dist

SERVICE      ?= corefusion
UNIT_SRC     := deploy/corefusion.service
UNIT_DST     := /etc/systemd/system/$(SERVICE).service

.PHONY: help all build panel panel-all panel-linux-amd64 panel-linux-arm64 backend frontend deps \
        agent agent-linux-amd64 agent-linux-arm64 \
        test vet fmt version run start stop restart status logs \
        service-install service-uninstall clean distclean

# panel 的 frontend 必须先于 backend（go:embed 依赖前端产物），禁止并行
.NOTPARALLEL:

help:
	@echo "CoreFusion 构建与运行（当前版本 $(VERSION)）"
	@echo ""
	@echo "  构建："
	@echo "    make panel              前端 + 本机架构面板（./corefusion）"
	@echo "    make panel-linux-amd64  交叉编译 linux-amd64 面板 → dist/"
	@echo "    make panel-linux-arm64  交叉编译 linux-arm64 面板 → dist/"
	@echo "    make panel-all          编译 amd64 + arm64 面板"
	@echo "    make backend            仅面板后端（embed 目录为空时自动构建前端）"
	@echo "    make frontend           仅构建前端并同步到 embed 目录"
	@echo "    make agent              编译 linux-amd64 Agent"
	@echo "    make agent-linux-arm64  交叉编译 linux-arm64 Agent"
	@echo ""
	@echo "  检查："
	@echo "    make test / vet / fmt"
	@echo ""
	@echo "  服务（systemd）："
	@echo "    make service-install    安装/更新服务并设为开机启动"
	@echo "    make service-uninstall  停用并删除服务"
	@echo "    make start / stop / restart / status / logs"
	@echo ""
	@echo "  其他："
	@echo "    make run                前台启动面板（调试用）"
	@echo ""
	@echo "  清理："
	@echo "    make clean              删除二进制与前端 dist"
	@echo "    make distclean          连同 embed 产物一起清理"
	@echo ""
	@echo "  构建参数（可覆盖）："
	@echo "    make test GOBUILDFLAGS= LDFLAGS=\"-X corefusion/internal/constants.Version=$(VERSION)\""
	@echo "                          调试用：保留符号表/本地路径"
	@echo "    默认 CGO_ENABLED=$(CGO_ENABLED)（静态链接）、-trimpath、-s -w -buildid="

all: panel

build: panel

# 面板 = 前端 + 后端（前端通过 go:embed 打进二进制）
panel: frontend backend

# 首次构建或 node_modules 缺失时安装依赖
deps:
	@test -d $(FRONTEND_DIR)/node_modules || (cd $(FRONTEND_DIR) && $(NPM) ci)

frontend: deps
	cd $(FRONTEND_DIR) && $(NPM) run build
	rm -rf $(EMBED_DIR)
	mkdir -p $(EMBED_DIR)
	cp -r $(FRONTEND_DIR)/dist/. $(EMBED_DIR)/

backend:
	@if [ ! -d $(EMBED_DIR) ] || [ -z "$$(ls -A $(EMBED_DIR) 2>/dev/null)" ]; then \
		echo "embed 目录为空，先构建前端..."; \
		$(MAKE) frontend; \
	fi
	$(GO) build $(GOBUILDFLAGS) -ldflags "$(LDFLAGS)" -o $(BINARY) ./cmd/corefusion/

# 交叉编译面板：前端同样通过 go:embed 打进二进制
panel-linux-amd64: frontend
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build $(GOBUILDFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/corefusion-linux-amd64 ./cmd/corefusion/

panel-linux-arm64: frontend
	@mkdir -p $(DIST_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build $(GOBUILDFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(DIST_DIR)/corefusion-linux-arm64 ./cmd/corefusion/

panel-all: panel-linux-amd64 panel-linux-arm64

agent: agent-linux-amd64

agent-linux-amd64:
	@mkdir -p $(AGENT_DIR)
	GOOS=linux GOARCH=amd64 $(GO) build -tags agent $(GOBUILDFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(AGENT_DIR)/corefusion-agent-linux-amd64 ./cmd/agent/

agent-linux-arm64:
	@mkdir -p $(AGENT_DIR)
	GOOS=linux GOARCH=arm64 $(GO) build -tags agent $(GOBUILDFLAGS) -ldflags "$(LDFLAGS)" \
		-o $(AGENT_DIR)/corefusion-agent-linux-arm64 ./cmd/agent/

test:
	$(GO) test ./...

vet:
	$(GO) vet ./...

fmt:
	$(GO) fmt ./...

version:
	@echo $(VERSION)

run:
	@test -x $(BINARY) || { echo "二进制不存在，请先 make panel" >&2; exit 1; }
	./$(BINARY)

service-install:
	@test -x $(BINARY) || { echo "二进制不存在，请先 make panel" >&2; exit 1; }
	sed -e 's#^WorkingDirectory=.*#WorkingDirectory=$(CURDIR)#' \
	    -e 's#^ExecStart=.*#ExecStart=$(CURDIR)/$(BINARY)#' \
	    $(UNIT_SRC) > $(UNIT_DST)
	chmod 644 $(UNIT_DST)
	systemctl daemon-reload
	systemctl enable $(SERVICE)
	systemctl restart $(SERVICE)
	@echo "已安装并启动：$(UNIT_DST)"

service-uninstall:
	-systemctl disable --now $(SERVICE)
	rm -f $(UNIT_DST)
	systemctl daemon-reload

start:
	systemctl start $(SERVICE)

stop:
	systemctl stop $(SERVICE)

restart:
	systemctl restart $(SERVICE)

status:
	systemctl status $(SERVICE) --no-pager --lines=20

logs:
	journalctl -u $(SERVICE) -n 200 -f

clean:
	rm -f $(BINARY)
	rm -rf $(DIST_DIR)
	rm -rf $(FRONTEND_DIR)/dist

distclean: clean
	rm -rf $(EMBED_DIR)
