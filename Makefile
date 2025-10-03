OUT := territories-referee
LDFLAGS = -s -w
GOTAGS := sqlite_math_functions
GOFLAGS = -v -trimpath -tags=$(GOTAGS) -ldflags="$(LDFLAGS)"
GO := go
CMD_PACKAGE = ./cmd/territories-referee
TESTFLAGS := $(GOFLAGS) -cover
SQLITE3_LIB := $(shell pkg-config --libs sqlite3)

.PHONY: require_sqlite3 build test clean

build: require_sqlite3
	$(GO) build -o $(OUT) $(GOFLAGS) $(CMD_PACKAGE)

test: require_sqlite3
	$(GO) test $(TESTFLAGS) ./cmd/... ./pkg/...

clean:
	rm -fv out/map* $(OUT)*

require_sqlite3:
ifndef SQLITE3_LIB
	$(error "territories-referee requires SQLite3")
endif