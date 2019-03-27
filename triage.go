package refs

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"strconv"
	"time"

	units "github.com/docker/go-units"

	"github.com/jcgregorio/webmention-func/admin"
	"github.com/jcgregorio/webmention-func/config"
	"github.com/jcgregorio/webmention-func/mention"
)

var (
	m *mention.Mentions

	triageTemplate = template.Must(template.New("triage").Funcs(template.FuncMap{
		"trunc": func(s string) string {
			if len(s) > 80 {
				return s[:80] + "..."
			}
			return s
		},
		"humanTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return " • " + units.HumanDuration(time.Now().Sub(t)) + " ago"
		},
	}).Parse(fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <title></title>
    <meta charset="utf-8" />
    <meta http-equiv="X-UA-Compatible" content="IE=egde,chrome=1">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <meta name="google-signin-scope" content="profile email">
    <meta name="google-signin-client_id" content="%s">
    <script src="https://apis.google.com/js/platform.js" async defer></script>
		<style type="text/css" media="screen">
		  #webmentions {
				display: grid;
				padding: 1em;
				grid-template-columns: 5em 10em 1fr;
				grid-column-gap: 10px;
				grid-row-gap: 6px;
			}
		</style>
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
  <div id=webmentions>
  {{range .Mentions }}
		<select name="text" data-key="{{ .Key }}">
			<option value="good" {{if eq .State "good" }}selected{{ end }} >Good</option>
			<option value="spam" {{if eq .State "spam" }}selected{{ end }} >Spam</option>
			<option value="untriaged" {{if eq .State "untriaged" }}selected{{ end }} >Untriaged</option>
		</select>
		<span>{{ .TS | humanTime }}</span>
		<div>
		  <div>Source: <a href="{{ .Source }}">{{ .Source | trunc }}</a></div>
			<div>Target: <a href="{{ .Target }}">{{ .Target | trunc }}</a></div>
		</div>
  {{end}}
  </div>
	<div><a href="?offset={{.Offset}}">Next</a></div>
	<script type="text/javascript" charset="utf-8">
	 // TODO - listen on div.webmentions for click/input and then write
	 // triage action back to server.
	 document.getElementById('webmentions').addEventListener('change', e => {
		 console.log(e);
		 if (e.target.dataset.key != "") {
			 fetch("/UpdateMention", {
			   credentials: 'same-origin',
				 method: 'POST',
				 body: JSON.stringify({
					 key: e.target.dataset.key,
					 value:  e.target.value,
				 }),
				 headers: new Headers({
					 'Content-Type': 'application/json'
				 })
			 }).catch(e => console.error('Error:', e));
		 }
	 });
	</script>
</body>
</html>`, config.CLIENT_ID)))

	mentionsTemplate = template.Must(template.New("mentions").Funcs(template.FuncMap{
		"humanTime": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return " • " + units.HumanDuration(time.Now().Sub(t)) + " ago"
		},
		"rfc3999": func(t time.Time) string {
			if t.IsZero() {
				return ""
			}
			return t.Format(time.RFC3339)
		},
		"trunc": func(s string) string {
			if len(s) > 200 {
				return s[:200] + "..."
			}
			return s
		},
	}).Parse(`
	<section id=webmention>
	<h3>WebMentions</h3>
	{{ range . }}
	    <span class="wm-author">
				{{ if .AuthorURL }}
					{{ if .Thumbnail }}
					<a href="{{ .AuthorURL}}" rel=nofollow class="wm-thumbnail">
						<img src="/u/thumbnail/{{ .Thumbnail }}"/>
					</a>
					{{ end }}
					<a href="{{ .AuthorURL}}" rel=nofollow>
						{{ .Author }}
					</a>
				{{ else }}
					{{ .Author }}
				{{ end }}
			</span>
			<time datetime="{{ .Published | rfc3999 }}">{{ .Published | humanTime }}</time>
			<a class="wm-content" href="{{ .Source }}" rel=nofollow>
				{{ if .Title }}
					{{ .Title | trunc }}
				{{ else }}
					{{ .Source | trunc }}
				{{ end }}
			</a>
	{{ end }}
	</section>
`))
)

func init() {
	var err error
	m, err = mention.NewMentions(context.Background(), config.PROJECT, config.DATASTORE_NAMESPACE)
	if err != nil {
		log.Fatal(err)
	}
}

type triageContext struct {
	IsAdmin  bool
	Mentions []*mention.MentionWithKey
	Offset   int64
}

// Triage displays the triage page for Webmentions.
func Triage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	context := &triageContext{}
	isAdmin := admin.IsAdmin(r)
	if isAdmin {
		limitText := r.FormValue("limit")
		if limitText == "" {
			limitText = "20"
		}
		offsetText := r.FormValue("offset")
		if offsetText == "" {
			offsetText = "0"
		}
		limit, err := strconv.ParseInt(limitText, 10, 32)
		if err != nil {
			log.Printf("Failed to parse limit: %s", err)
			return
		}
		offset, err := strconv.ParseInt(offsetText, 10, 32)
		if err != nil {
			log.Printf("Failed to parse offset: %s", err)
			return
		}
		context = &triageContext{
			IsAdmin:  isAdmin,
			Mentions: m.GetTriage(r.Context(), int(limit), int(offset)),
			Offset:   offset + limit,
		}
	}
	if err := triageTemplate.Execute(w, context); err != nil {
		log.Printf("Failed to render triage template: %s", err)
	}
}

type updateMention struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

func UpdateMention(w http.ResponseWriter, r *http.Request) {
	isAdmin := admin.IsAdmin(r)
	if !isAdmin {
		http.Error(w, "Unauthorized", 401)
	}
	var u updateMention
	if err := json.NewDecoder(r.Body).Decode(&u); err != nil {
		log.Printf("Failed to decode update: %s", err)
		http.Error(w, "Bad JSON", 400)
	}
	if err := m.UpdateState(r.Context(), u.Key, u.Value); err != nil {
		log.Printf("Failed to write update: %s", err)
		http.Error(w, "Failed to write", 400)
	}
}

// Mentions returns HTML describing all the good Webmentions for the given URL.
func Mentions(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	m := m.GetGood(r.Context(), r.Referer())
	if len(m) == 0 {
		return
	}
	if err := mentionsTemplate.Execute(w, m); err != nil {
		log.Printf("Failed to expand template: %s", err)
	}
}
