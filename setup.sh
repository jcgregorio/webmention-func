set -x -e

PROJECT=heroic-muse-88515

gcloud --project=${PROJECT} pubsub topics create webmention-validate
gcloud --project=${PROJECT} beta scheduler jobs create pubsub webmention-validate --schedule="* * * * *" --topic=webmention-validate --message-body="v"
