# Regenerates marketing screenshots against a throwaway stack seeded with
# fictional data. Run `make` here; everything is torn down afterwards.

ROOT := $(abspath ../../..)
HERE := $(abspath .)
TMP := $(HERE)/tmp

CONTAINER ?= podman
DB_CTR := sickrock-mkt-mariadb
DB_PORT := 13306

BACKEND_URL := http://127.0.0.1:18480

# Antora serves images from the module's images/ directory.
DOCS_IMAGES := $(ROOT)/docs/modules/ROOT/images/marketing
DOCS_PNGS := table.png row.png insert-row.png calendar.png home.png control-panel.png table-dark.png

.PHONY: all build up seed shoot manifest publish down clean

all:
	@$(MAKE) build up seed shoot manifest publish; rc=$$?; $(MAKE) down; exit $$rc

build:
	$(MAKE) -C $(ROOT)/service build
	[ -d $(ROOT)/frontend/node_modules ] || (cd $(ROOT)/frontend && npm ci)
	cd $(ROOT)/frontend && npx vite build

up:
	mkdir -p $(TMP)/service
	cp service.yaml $(TMP)/service/config.yaml
	cp SickRock.env $(TMP)/service/SickRock.env
	$(CONTAINER) run -d --rm --name $(DB_CTR) -p 127.0.0.1:$(DB_PORT):3306 \
		-e MARIADB_ROOT_PASSWORD=root-demo -e MARIADB_DATABASE=sickrock \
		-e MARIADB_USER=sickrock -e MARIADB_PASSWORD=sickrock-demo \
		docker.io/library/mariadb:11.4
	@echo "Waiting for MariaDB..."
	@for i in $$(seq 120); do \
		$(CONTAINER) exec $(DB_CTR) mariadb -usickrock -psickrock-demo -h127.0.0.1 -e 'SELECT 1' sickrock >/dev/null 2>&1 && exit 0; \
		sleep 1; done; echo "MariaDB did not start"; exit 1
	# Migrations are found relative to the working directory (service/).
	cd $(ROOT)/service && { SICKROCK_FRONTEND_DIR=$(ROOT)/frontend/dist ./tmp/sickrock -configdir $(TMP)/service \
		> $(TMP)/service.log 2>&1 & echo $$! > $(TMP)/service.pid; }
	@for i in $$(seq 60); do curl -fsS -X POST -H 'Content-Type: application/json' \
		-d '{"username":"admin","password":"admin"}' $(BACKEND_URL)/api/sickrock.SickRock/Login >/dev/null 2>&1 && exit 0; \
		sleep 1; done; echo "Backend did not become ready; see $(TMP)/service.log"; exit 1

seed:
	./seed.sh $(BACKEND_URL)

shoot:
	repo-helper screenshot --config screenshots.ini

manifest:
	./generate-manifest.py

publish:
	mkdir -p $(DOCS_IMAGES)
	cp $(DOCS_PNGS) $(DOCS_IMAGES)/

down:
	-@[ -f $(TMP)/service.pid ] && kill $$(cat $(TMP)/service.pid) 2>/dev/null; rm -f $(TMP)/service.pid
	-@$(CONTAINER) rm -f $(DB_CTR) >/dev/null 2>&1

clean: down
	rm -rf $(TMP)
