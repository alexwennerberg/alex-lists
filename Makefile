PREFIX?=/usr/local
BINDIR?=$(PREFIX)/bin
LIBDIR?=$(PREFIX)/lib
SHAREDIR?=$(PREFIX)/share

ASSETS=$(SHAREDIR)/sourcehut

SERVICE=lists.sr.ht
STATICDIR=$(ASSETS)/static/$(SERVICE)

BINARIES=\
	$(SERVICE)-ingress \
	$(SERVICE)-web

all: all-bin all-share

install: install-bin install-share

clean: clean-bin clean-share

all-bin: $(BINARIES)

all-share: static/main.min.css

install-bin: all-bin
	mkdir -p $(BINDIR)
	for bin in $(BINARIES); \
	do \
		install -Dm755 $$bin $(BINDIR)/; \
	done

install-share: all-share
	mkdir -p $(STATICDIR)
	install -Dm644 static/*.css $(STATICDIR)
	install -Dm644 schema.sql $(ASSETS)/$(SERVICE).sql

clean-bin:
	rm -f $(BINARIES)

clean-share:
	rm -f static/main.min.css static/main.min.*.css

.PHONY: all all-bin all-share
.PHONY: install install-bin install-share
.PHONY: clean clean-bin clean-share

# static/main.css is hand-written and checked in; only minification is built.
static/main.min.css: static/main.css
	minify -o $@ $<
	cp $@ $(@D)/main.min.$$(sha256sum $@ | cut -c1-8).css

$(SERVICE)-ingress:
	go build -o $@ ./ingress

$(SERVICE)-web:
	go build -o $@ ./web

# Always rebuild
.PHONY: $(BINARIES)
