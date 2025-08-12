.PHONY: all proto proto-generate proto-lint proto-clean service service-build service-run service-test service-clean frontend frontend-dev frontend-build frontend-preview

all: proto-generate service-build

proto:
	$(MAKE) -wC proto

proto-generate:
	$(MAKE) -wC proto generate

proto-lint:
	$(MAKE) -wC proto lint

proto-clean:
	$(MAKE) -wC proto clean


# Service targets
service:
	$(MAKE) -wC service

service-build:
	$(MAKE) -wC service build

service-run:
	$(MAKE) -wC service run

service-test:
	$(MAKE) -wC service test

service-clean:
	$(MAKE) -wC service clean

# Frontend targets
frontend:
	$(MAKE) -wC frontend dev

frontend-dev:
	$(MAKE) -wC frontend dev

frontend-build:
	$(MAKE) -wC frontend build

frontend-preview:
	$(MAKE) -wC frontend preview

