.PHONY: all proto proto-generate proto-lint proto-clean service service-build service-run service-test service-clean frontend frontend-build frontend-preview icons icons-clean

all: proto-generate frontend-build service-build

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

service-prep:
	$(MAKE) -wC service prep

# Frontend targets
frontend:
	$(MAKE) -wC frontend

frontend-build:
	$(MAKE) -wC frontend build

frontend-preview:
	$(MAKE) -wC frontend preview

# Icon generation from SVG
ICON_SIZES = 72 96 128 144 152 192 384 512
ICON_DIR = frontend/public/icons
SVG_SOURCE = logo.svg

icons: $(foreach size,$(ICON_SIZES),$(ICON_DIR)/icon-$(size)x$(size).png)

$(ICON_DIR)/icon-%.png: $(SVG_SOURCE)
	@echo "Generating icon: $@"
	@mkdir -p $(ICON_DIR)
	@size=$$(echo $* | sed 's/x.*//'); \
	if [ -z "$$size" ]; then \
		echo "Error: Could not extract size from pattern $*"; \
		exit 1; \
	fi; \
	inkscape --export-filename="$@" --export-width=$$size --export-height=$$size "$<"

icons-clean:
	@echo "Cleaning generated icons..."
	@rm -f $(foreach size,$(ICON_SIZES),$(ICON_DIR)/icon-$(size)x$(size).png)
	@echo "Icons cleaned."
