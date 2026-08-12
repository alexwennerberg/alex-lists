PREFIX?=/usr/local
BINDIR?=$(PREFIX)/bin
LIBDIR?=$(PREFIX)/lib
SHAREDIR?=$(PREFIX)/share

ASSETS=$(SHAREDIR)/sourcehut

SERVICE=lists.sr.ht
STATICDIR=$(ASSETS)/static/$(SERVICE)
MIGRATIONDIR=$(ASSETS)/migrations/$(SERVICE)

SASSC?=sassc
SASSC_INCLUDE=-I$(ASSETS)/scss/

BINARIES=\
	$(SERVICE)-ingress

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
	mkdir -p $(MIGRATIONDIR)
	install -Dm644 static/*.css $(STATICDIR)
	install -Dm644 schema.sql $(ASSETS)/$(SERVICE).sql
	install -Dm644 migrations/*.sql $(MIGRATIONDIR)

clean-bin:
	rm -f $(BINARIES)

clean-share:
	rm -f static/main.min.css static/main.css

.PHONY: all all-bin all-share
.PHONY: install install-bin install-share
.PHONY: clean clean-bin clean-share

static/main.css: scss/main.scss
	mkdir -p $(@D)
	$(SASSC) $(SASSC_INCLUDE) $< $@

static/main.min.css: static/main.css
	minify -o $@ $<
	cp $@ $(@D)/main.min.$$(sha256sum $@ | cut -c1-8).css

$(SERVICE)-ingress:
	go build -o $@ ./ingress

# Always rebuild
.PHONY: $(BINARIES)
