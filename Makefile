include ./Makefile.Common

# Every module in the repository, discovered rather than listed so a new one is
# picked up without editing this file. internal/tools is excluded: it holds no
# packages, only tool pins.
FIND_MOD_ARGS = -type f -name "go.mod"
EX_TOOLS = -not -path "./internal/tools/*"

# Includes the root module as ".", so `make -C . <target>` picks up the same
# per-module targets from Makefile.Common that the others use.
ALL_MODS := $(shell find . $(EX_TOOLS) $(FIND_MOD_ARGS) -exec dirname {} \; | sort)

.PHONY: all
all: gomoddownload golint gotest

# Run $(TARGET) inside every module.
.PHONY: $(ALL_MODS)
$(ALL_MODS):
	@echo "Running target '$(TARGET)' in module '$@'"
	@$(MAKE) --no-print-directory -C $@ $(TARGET)

.PHONY: for-all-target
for-all-target: $(ALL_MODS)

.PHONY: gotest
gotest:
	@$(MAKE) for-all-target TARGET="test"

.PHONY: gotest-with-cover
gotest-with-cover:
	@$(MAKE) for-all-target TARGET="test-with-cover"

.PHONY: gobench
gobench:
	@$(MAKE) for-all-target TARGET="bench"

.PHONY: golint
golint:
	@$(MAKE) for-all-target TARGET="lint"

.PHONY: golint-fix
golint-fix:
	@$(MAKE) for-all-target TARGET="lint-fix"

.PHONY: gofmt
gofmt:
	@$(MAKE) for-all-target TARGET="fmt"

.PHONY: gotidy
gotidy:
	@$(MAKE) for-all-target TARGET="tidy"
	@cd $(TOOLS_MOD_DIR) && GOWORK=off $(GOCMD) mod tidy

.PHONY: gomoddownload
gomoddownload:
	@$(MAKE) for-all-target TARGET="moddownload"

.PHONY: gogovulncheck
gogovulncheck:
	@$(MAKE) for-all-target TARGET="govulncheck"

.PHONY: goaddlicense
goaddlicense:
	@$(MAKE) for-all-target TARGET="addlicense"

.PHONY: gochecklicense
gochecklicense:
	@$(MAKE) for-all-target TARGET="checklicense"

# Build every package, including ones no test imports.
.PHONY: gobuild
gobuild:
	@set -e; for mod in $(ALL_MODS); do \
		echo "Building '$$mod'"; \
		(cd $$mod && $(GOCMD) build ./...); \
	done

.PHONY: precommit
precommit: gofmt gotidy golint gotest gobuild

.PHONY: modules
modules:
	@echo $(ALL_MODS) | tr ' ' '\n'
