LDFLAGS = -s -w
GOTAGS = sqlite_math_functions
GOFLAGS = -v -trimpath -tags "$(GOTAGS)" -ldflags="$(LDFLAGS)"
GO := go
CMD_PACKAGE = ./cmd/territories-referee

SQLITE3_LIB := $(shell pkg-config --libs sqlite3)

.PHONY: build test clean

build:
	$(GO) build $(GOFLAGS) $(CMD_PACKAGE)

test:
	$(GO) test $(GOFLAGS) ./cmd/... ./pkg/...

clean:
	rm -fv out/map*
