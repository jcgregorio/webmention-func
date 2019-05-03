include ./config.mk
run:
	printenv
	go run ./webmention.go


release:
	CGO_ENABLED=0 GOOS=linux go install -a ./webmention.go



