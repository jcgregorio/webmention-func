include ./config.mk
default:
	go build .
	echo gcloud --project=$(PROJECT) --region=$(REGION) functions deploy Triage --runtime go111 --trigger-http

deploy:
	gcloud functions deploy UpdateMention --runtime go111 --trigger-http
	gcloud functions deploy Mentions --runtime go111 --trigger-http
	gcloud functions deploy IncomingWebMention --runtime go111 --trigger-http
	gcloud functions deploy Thumbnail --runtime go111 --trigger-http
	gcloud functions deploy VerifyQueuedMentions --runtime go111 --trigger-topic=webmention-validate

