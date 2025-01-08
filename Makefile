SRHT_PATH?=/usr/lib/python3.12/site-packages/srht
MODULE=listssrht/
include ${SRHT_PATH}/Makefile

all: build

build:
	cd api && go generate ./loaders
	cd api && go generate ./graph
	cd api && go build
	cd ingress && go build

.PHONY: all build
