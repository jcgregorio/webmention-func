SHELL=/bin/bash
include ./config.mk
run:
	printenv
	go run ./webmention.go


release:
	CGO_ENABLED=0 GOOS=linux go install -a ./webmention.go
	mkdir -p ./build
	rm -rf ./build/*
	cp $(GOPATH)/bin/webmention ./build
	cp Dockerfile ./build
	docker build ./build --tag webmention --tag gcr.io/$(PROJECT)/webmention
	docker push gcr.io/$(PROJECT)/webmention

run_release:
	./run_release.sh

push:
	gcloud beta run deploy webmention --allow-unauthenticated --region $(REGION) --image gcr.io/$(PROJECT)/webmention --set-env-vars "$(shell cat config.mk | sed 's#export ##' | grep -v "^PORT=" | tr '\n' ',')"


