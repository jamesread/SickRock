.PHONY: all deps build preview test lint ci

all: build

ci: deps build

deps:
	npm ci

build:
	npx vite build

preview:
	npx vite preview

test:
	npm run test

lint:
	npm run lint
