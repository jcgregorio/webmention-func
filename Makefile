default:
	go build .

deploy:
	gcloud functions deploy Triage --runtime go111 --trigger-http
	gcloud functions deploy UpdateMention --runtime go111 --trigger-http
