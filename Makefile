# trunkline — сборка CLI. Ядро, профили и agent-skills вшиты в бинарь через
# go:embed (assets.go), поэтому на выходе всегда один самодостаточный файл.
#
#   make build     — локальная сборка в ./bin (ничего не устанавливает)
#   make install   — собрать в свою папку и предложить добавить в PATH себе
#   make uninstall — обратное для install
#   make release   — кросс-сборка артефактов в ./dist для раздачи другим
#
# Все цели требуют установленного Go. Пользователю без Go отдаётся готовый
# бинарь из dist/ — ему тулчейн уже не нужен.

.PHONY: build install uninstall release test vet tidy clean

# Версия ядра. Тот же файл вшивается в бинарь (`//go:embed VERSION`), и именно
# её сравнивает `trunkline update` в проекте.
VERSION := $(shell cat VERSION)

# Платформы для `release`. Добавить windows/arm64 и т.п. — просто дописать сюда.
PLATFORMS := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64

## build: собрать в ./bin/trunkline — проверить компиляцию, погонять локально.
## На то, что запускается по имени `trunkline`, не влияет: для этого install.
build:
	go build -o bin/trunkline ./cmd/trunkline

## install: собрать бинарь в свою папку (по умолчанию ~/.trunkline/bin, не
## GOPATH/bin — не все пользователи держат её в PATH) и интерактивно
## предложить добавить эту папку в PATH текущего шелла. См. scripts/install.sh
## (Windows: scripts/install.ps1). Обратное действие: `make uninstall`.
install:
	@sh scripts/install.sh

## uninstall: обратное для install — удалить бинарь и PATH-блок, который
## install мог дописать в rc-файл шелла. См. scripts/uninstall.sh
## (Windows: scripts/uninstall.ps1).
uninstall:
	@sh scripts/uninstall.sh

## release: в отличие от install, ничего никуда не устанавливает и не трогает
## PATH — только кросс-компилирует бинарь на каждую платформу из PLATFORMS
## в ./dist, для раздачи другим (CI-артефакт, вложение в релиз и т.п.).
## По бинарю на платформу в ./dist + SHA256SUMS.
## CGO_ENABLED=0 — статическая линковка и кросс-компиляция без чужих тулчейнов.
## -trimpath убирает из бинаря абсолютные пути сборки (/home/<user>/...),
## -s -w выбрасывает debug-символы: ~4.7 МБ вместо ~6.3 МБ.
## Зависит от clean, чтобы в dist/ не оставались артефакты прошлой версии.
release: clean
	@mkdir -p dist
	@for p in $(PLATFORMS); do \
		os=$${p%/*}; arch=$${p#*/}; \
		out="dist/trunkline_$(VERSION)_$${os}_$${arch}"; \
		[ "$$os" = windows ] && out="$$out.exe"; \
		echo "GOOS=$$os GOARCH=$$arch -> $$out"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
			go build -trimpath -ldflags "-s -w" -o "$$out" ./cmd/trunkline || exit 1; \
	done
	@cd dist && sha256sum trunkline_* > SHA256SUMS
	@ls -l dist

## test: go test по всему дереву.
test:
	go test ./...

## vet: go vet по всему дереву. Вместе с build — гейт для PR (docs/RELEASE-CHECKLIST.md).
vet:
	go vet ./...

## tidy: привести go.mod/go.sum в порядок.
tidy:
	go mod tidy

## clean: снести артефакты сборки. Установленную копию в PATH не трогает.
clean:
	rm -rf bin dist
