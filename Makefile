.DEFAULT_GOAL := build
HAS_UPX := $(shell command -v upx 2> /dev/null)

.PHONY: build
build:
	go build -ldflags="-X main.version=v2-`git rev-parse --short HEAD`" -o ./feishu2md cmd/*.go
ifneq ($(and $(COMPRESS),$(HAS_UPX)),)
	upx -9 ./feishu2md
endif

.PHONY: test
test:
	go test ./core/... ./web/...

.PHONY: server
server:
	go build -o ./feishu2md-server web/*.go

.PHONY: flutter
flutter:
	cd feishu2md_app && flutter build windows

.PHONY: flutter-web
flutter-web:
	cd feishu2md_app && flutter build web

.PHONY: all-in-one
all-in-one: server flutter
	@echo "构建完整应用程序"
	mkdir -p ./dist
	cp ./feishu2md-server ./dist/
	cp -r ./feishu2md_app/build/windows/runner/Release/* ./dist/

.PHONY: web-app
web-app: server flutter-web
	@echo "构建Web应用程序"
	mkdir -p ./dist/web
	cp ./feishu2md-server ./dist/
	cp -r ./feishu2md_app/build/web/* ./dist/web/

.PHONY: image
image:
	docker build -t feishu2md .

.PHONY: docker
docker:
	docker run -it --rm -p 8080:8080 feishu2md

.PHONY: clean
clean:  ## 清理构建产物
	rm -f ./feishu2md ./feishu2md-server
	rm -rf ./dist

.PHONY: format
format:
	gofmt -l -w .

.PHONY: all
all: build server
	@echo "构建完成"
