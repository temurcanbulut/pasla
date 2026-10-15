VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS := -s -w -X main.version=$(VERSION)
GOFLAGS := -trimpath
TARGETS := windows/amd64 windows/arm64 linux/amd64 linux/arm64

.PHONY: build run test vet fuzz release clean winres site

build:
	go build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o dist/pasla ./cmd/pasla

run: build
	./dist/pasla

test:
	go test -race -count=1 ./...

fuzz:
	go test -run=XXX -fuzz=FuzzSanitizeRelPath -fuzztime=60s ./internal/node

vet:
	go vet ./...
	GOOS=windows go vet ./...

winres:
	go run ./tools/winres -version $(VERSION) -icon cmd/pasla/icon.ico -out cmd/pasla

release: clean winres
	@for t in $(TARGETS); do \
		os=$${t%/*}; arch=$${t#*/}; ext=""; [ $$os = windows ] && ext=".exe"; \
		echo "→ $$os/$$arch"; \
		ld="$(LDFLAGS)"; [ $$os = windows ] && ld="$$ld -H windowsgui"; \
		CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch go build $(GOFLAGS) -ldflags "$$ld" \
			-o dist/pasla-$$os-$$arch$$ext ./cmd/pasla || exit 1; \
	done
	cd dist && sha256sum pasla-* > SHA256SUMS.txt

clean:
	rm -rf dist

# The GitHub Pages site, assembled into dist/site. Preview it with
#   python3 -m http.server -d dist/site
site:
	sh site/build.sh dist/site
