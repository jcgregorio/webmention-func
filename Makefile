default:
	go build .

deploy:
	gcloud functions deploy Triage --runtime go111 --trigger-http
	gcloud functions deploy UpdateMention --runtime go111 --trigger-http
	gcloud functions deploy Mentions --runtime go111 --trigger-http
	gcloud functions deploy IncomingWebMention --runtime go111 --trigger-http
	gcloud functions deploy VerifyQueuedMentions --runtime go111 --trigger-topic=webmention-validate

