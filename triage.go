package refs

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"

	"github.com/jcgregorio/webmention-func/admin"
	"github.com/jcgregorio/webmention-func/config"
	"github.com/jcgregorio/webmention-func/mention"
)

var (
	m           *mention.Mentions
	refTemplate *template.Template
	refSource   = fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title></title>
    <meta charset="utf-8" />
    <meta http-equiv="X-UA-Compatible" content="IE=egde,chrome=1">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="google-signin-scope" content="profile email">
    <meta name="google-signin-client_id" content="%s">
    <script src="https://apis.google.com/js/platform.js" async defer></script>
</head>
<body>
  <div class="g-signin2" data-onsuccess="onSignIn" data-theme="dark"></div>
	<script>
	function onSignIn(googleUser) {
		document.cookie = "id_token=" + googleUser.getAuthResponse().id_token;
		if (!{{.IsAdmin}}) {
			window.location.reload();
		}
	};
	</script>
	<h1>Content goes here.</h1>
</body>
</html>
`, config.CLIENT_ID)
)

func init() {
	refTemplate = template.Must(template.New("ref").Parse(refSource))
	var err error
	m, err = mention.NewMentions(context.Background(), config.PROJECT, config.DATASTORE_NAMESPACE)
	if err != nil {
		log.Fatal(err)
	}
}

type refPageContext struct {
	IsAdmin bool
}

func TriageHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	if err := refTemplate.Execute(w, refPageContext{
		IsAdmin: admin.IsAdmin(r),
	}); err != nil {
		log.Printf("Failed to render ref template: %s", err)
	}
}
