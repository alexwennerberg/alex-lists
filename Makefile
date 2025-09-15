PREFIX?=/usr/local
BINDIR?=$(PREFIX)/bin
LIBDIR?=$(PREFIX)/lib
SHAREDIR?=$(PREFIX)/share

ASSETS=$(SHAREDIR)/sourcehut

SERVICE=lists.sr.ht
STATICDIR=$(ASSETS)/static/$(SERVICE)

SASSC?=sassc
SASSC_INCLUDE=-I$(ASSETS)/scss/

ARIADNE_CODEGEN=ariadne-codegen

BINARIES=\
	$(SERVICE)-api \
	$(SERVICE)-ingress

all: all-bin all-share all-python

install: install-bin install-share

clean: clean-bin clean-share clean-python

all-bin: $(BINARIES)

all-share: static/main.min.css

GRAPHQL_QUERIES != echo listssrht/graphql/*.graphql

listssrht/graphql/__init__.py: pyproject.toml $(GRAPHQL_QUERIES)
	$(ARIADNE_CODEGEN)

all-python: listssrht/graphql/__init__.py

install-bin: all-bin
	mkdir -p $(BINDIR)
	for bin in $(BINARIES); \
	do \
		install -Dm755 $$bin $(BINDIR)/; \
	done

install-share: all-share
	mkdir -p $(STATICDIR)
	install -Dm644 static/*.css $(STATICDIR)
	install -Dm644 api/graph/schema.graphqls $(ASSETS)/$(SERVICE).graphqls

clean-bin:
	rm -f $(BINARIES)

clean-share:
	rm -f static/main.min.css static/main.css

clean-python:
	rm -rf listssrht/graphql/*.py listssrht/graphql/__pycache__

.PHONY: all all-bin all-share all-python
.PHONY: install install-bin install-share
.PHONY: clean clean-bin clean-share clean-python

static/main.css: scss/main.scss
	mkdir -p $(@D)
	$(SASSC) $(SASSC_INCLUDE) $< $@

static/main.min.css: static/main.css
	minify -o $@ $<
	cp $@ $(@D)/main.min.$$(sha256sum $@ | cut -c1-8).css

api/loaders/*_gen.go &: api/loaders/generate.go api/loaders/gen go.sum
	cd api && go generate ./loaders

api/graph/api/generated.go: api/graph/schema.graphqls api/graph/generate.go go.sum api/loaders/*_gen.go
	cd api && go generate ./graph

$(SERVICE)-api: api/graph/api/generated.go api/loaders/*_gen.go
	go build -o $@ ./api

$(SERVICE)-ingress:
	go build -o $@ ./ingress

# Always rebuild
.PHONY: $(BINARIES)
