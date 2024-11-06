SUBDIRS := cmd/rpcbench cmd/fuseexpr
all: build
$(SUBDIRS):
	$(MAKE) -C $@

.PHONY: all $(SUBDIRS)

build: $(SUBDIRS)

run-server: build
	./bin/bench server --mode=$(mode)
