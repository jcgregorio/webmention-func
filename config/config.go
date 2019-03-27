package config

import "fmt"

const (
	CLIENT_ID           = "952643138919-jh0117ivtbqkc9njoh91csm7s465c4na.apps.googleusercontent.com"
	REGION              = "us-central1"
	PROJECT             = "heroic-muse-88515"
	DATASTORE_NAMESPACE = "blog"
)

var (
	HOST   = fmt.Sprintf("https://%s-%s.cloudfunctions.net", REGION, PROJECT)
	ADMINS = []string{"joe.gregorio@gmail.com"}
)
