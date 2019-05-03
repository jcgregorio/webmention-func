include ./config.mk
run:
	printenv
	go run ./webmention.go


release:
	#CGO_ENABLED=0 GOOS=linux go install -a ./webmention.go
	mkdir -p ./build
	rm -rf ./build/*
	cp $(GOPATH)/bin/webmention ./build
	cp Dockerfile ./build
	sudo docker build ./build --tag webmention



