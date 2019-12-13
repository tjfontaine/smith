NAME = smith
REMOTE = github.com/oracle

DIRS := \
	. \

uniq = $(if $1,$(firstword $1) $(call uniq,$(filter-out $(firstword $1),$1)))
gofiles = $(foreach d,$(1),$(wildcard $(d)/*.go))
testdirs = $(call uniq,$(foreach d,$(1),$(dir $(wildcard $(d)/*_test.go))))
fmt = $(addprefix fmt-,$(1))

all: $(NAME)

sha = $(shell git rev-parse --short HEAD || cat SHA | tr -d ' \n')
ifeq ($(VERSION),)
VERSION = $(shell git describe --tags --match 'v*.*.*' | tr -d 'v \n')
realv = $(shell printf $(VERSION) | cut -d'-' -f1)
ifneq ($(VERSION),$(realv))
commits = $(shell printf $(VERSION) | cut -d'-' -f2)
VERSION := $(realv).$(commits).$(sha)
endif
endif
dirty = $(shell git diff --shortstat 2> /dev/null | tail -n1 | tr -d ' \n')
ifneq ($(dirty),)
VERSION := $(VERSION).dev
endif
id = $(shell head -c20 /dev/urandom|od -An -tx1|tr -d ' \n')
# this complicated construction is to make sure that the project can be built
# if it is cloned outside of the gopath. Go's vendor support only works if
# the project is inside the gopath
$(NAME): $(call gofiles,$(DIRS))
	CGO_ENABLED=0 go build -a -x -v \
		-ldflags '-X "main.ver=$(VERSION)" -X "main.sha=$(sha)" -B 0x$(id)' \
		-o $(NAME) .

.PHONY: $(call testdirs,$(DIRS))
$(call testdirs,$(DIRS)):
	sudo -E $(GOPATH)/bin/godep go test -v $(REMOTE)/$(NAME)/$@

$(call fmt,$(call testdirs,$(DIRS))):
	! gofmt -l -w $(subst fmt-,,$@)*.go | grep ''

.PHONY: fmt
fmt: $(call fmt,$(call testdirs,$(DIRS)))

test: fmt $(call testdirs,$(DIRS))

clean:
	rm -f $(NAME)
	rm -rf build
	rm -rf rpm

install: all
	install -D smith $(DESTDIR)/usr/bin/smith

rpm:
	mkdir -p rpm

rpm/SPECS/smith.spec: rpm all
	mkdir -p rpm/SPECS
	sed s/@VERSION@/$(VERSION)/g smith.spec > rpm/SPECS/smith.spec

rpm/SOURCES/smith-$(VERSION).tar.gz: rpm
	mkdir -p rpm/SOURCES
	git archive -o rpm/SOURCES/smith-$(VERSION).tar.gz --prefix "smith-$(VERSION)/" HEAD

rpm/smith-$(VERSION)-3.src.rpm: rpm/SPECS/smith.spec rpm/SOURCES/smith-$(VERSION).tar.gz
	/usr/bin/mock --buildsrpm --spec rpm/SPECS/smith.spec --sources rpm/SOURCES --resultdir rpm

rpm/smith-$(VERSION)-3.x86_64.rpm: rpm/smith-$(VERSION)-3.src.rpm
	/usr/bin/mock --rebuild rpm/smith-$(VERSION)-3.src.rpm --resultdir rpm

.PHONY: rpms
rpms: rpm/smith-$(VERSION)-3.x86_64.rpm

.PHONY: version
version:
	@echo $(VERSION)

build-docker-clean:
	rm -rf from-docker-image.tar.gz from-smith-image.tar.gz checksum.txt

ORG := ${USER}
REPO := ${ORG}/${NAME}
TAG := ${REPO}:${VERSION}

build-docker-build:
	docker build -t ${TAG} .

build-docker-archive: build-docker-clean build-docker-build
	docker save ${TAG} | gzip -c > from-docker-image.tar.gz
	docker run \
			-it --rm \
			-v ${PWD}:/write \
			-v tmp:/tmp \
			${TAG} \
			--docker \
			-i from-smith-image.tar.gz \
			--tag ${TAG}
	docker load -i from-smith-image.tar.gz
	sha1sum from-smith-image.tar.gz > checksum.txt
	rm from-smith-image.tar.gz
	docker run \
			-it --rm \
			-v ${PWD}:/write \
			-v tmp:/tmp \
			${TAG} \
			--docker \
			-i from-smith-image.tar.gz \
			--tag ${TAG}
	sha1sum -c checksum.txt
	rm -rf from-docker-image.tar.gz from-smith-image.tar.gz checksum.txt
	docker tag ${TAG} ${REPO}:latest

build-docker: build-docker-archive build-docker-clean
