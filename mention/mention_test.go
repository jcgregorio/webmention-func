package mention

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"math/rand"
	"net"
	"net/http"
	"net/url"
	"os"
	"testing"
	"time"

	_ "image/gif"
	_ "image/jpeg"

	"github.com/stretchr/testify/assert"
	"willnorris.com/go/microformats"
)

const (
	feed1 = `<?xml version="1.0" encoding="utf-8"?>
<feed xmlns="http://www.w3.org/2005/Atom">
   <title type="html">BitWorking</title>
   <link href="http://bitworking.org/" />
   <link href="http://bitworking.org/news/feed/" rel="self" />
   <link rel="hub" href="https://pubsubhubbub.appspot.com/"/>
   <link rel="me" href="http://www.google.com/profiles/joe.gregorio" type="text/html" />
   <updated>2016-09-12T07:21:48-04:00</updated>
   <author>
      <name>Joe Gregorio</name>
   </author>
   <id>http://bitworking.org/</id>
   <entry>
     <title type="html">Inertial Balance</title>
     <link href="http://bitworking.org/news/2016/08/interial_balance" />
     <id>http://bitworking.org/news/2016/08/content2</id>
     <updated>2016-08-16T22:42:54-04:00</updated>
     <content type="html">This is the content &lt;a href=&#34;http://example.com&#34;&gt;</content>
   </entry>
   <entry>
     <updated>2016-08-16T14:30:50-04:00</updated>
     <id>http://bitworking.org/news/2016/08/stuff</id>
     <link href="http://bitworking.org/news/2016/08/stuff"/>
     <content type="html">This is stuff</content>
   </entry>
   <entry>
     <updated>2016-08-16T14:30:50-04:00</updated>
     <id>http://bitworking.org/news/2016/09/relative</id>
     <link href="http://bitworking.org/news/2016/08/relative"/>
     <content type="html">This is the content &lt;a href=&#34;sample.js&#34;&gt;</content>
   </entry>
</feed>`
)

func TestParseAtomFeed(t *testing.T) {
	buf := bytes.NewBufferString(feed1)
	mentionSources, err := ParseAtomFeed(buf)
	assert.NoError(t, err)
	assert.NotNil(t, mentionSources)
	assert.Equal(t, 3, len(mentionSources))
	/*
		assert.Equal(t, "http://example.com", mentionSources[0].Targets[0])
		assert.Equal(t, "http://bitworking.org/news/2016/08/interial_balance", mentionSources[0].Source)
		assert.Equal(t, 0, len(mentionSources[1].Targets))
		assert.Equal(t, "http://bitworking.org/news/2016/08/sample.js", mentionSources[2].Targets[0])
		assert.Equal(t, "http://bitworking.org/news/2016/08/sample.js", mentionSources[2].Targets[0])
	*/
}

// InitDatastore is a common utility function used in tests. It sets up the
// datastore to connect to the emulator and also clears out all instances of
// the given 'kinds' from the datastore.
func InitForTesting(t assert.TestingT) *Mentions {
	r := rand.New(rand.NewSource(time.Now().UnixNano()))
	emulatorHost := os.Getenv("DATASTORE_EMULATOR_HOST")
	if emulatorHost == "" {
		assert.Fail(t, `Running tests that require a running Cloud Datastore emulator.

Run

	"gcloud beta emulators datastore start --no-store-on-disk --host-port=localhost:8888"

and then run

  $(gcloud beta emulators datastore env-init)

to set the environment variables. When done running tests you can unset the env variables:

  $(gcloud beta emulators datastore env-unset)

`)
	}

	// Copied from net/http to create a fresh http client. In some tests the
	// httpmock replaces the default http client and the healthcheck below fails.
	var transport http.RoundTripper = &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
			DualStack: true,
		}).DialContext,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	httpClient := &http.Client{Transport: transport}

	// Do a quick healthcheck against the host, which will fail immediately if it's down.
	_, err := httpClient.Get("http://" + emulatorHost + "/")
	assert.NoError(t, err, fmt.Sprintf("Cloud emulator host %s appears to be down or not accessible.", emulatorHost))

	m, err := NewMentions(context.Background(), "test-project", fmt.Sprintf("test-namespace-%d", r.Uint64()))
	assert.NoError(t, err)
	return m
}

func TestDB(t *testing.T) {
	m := InitForTesting(t)

	err := m.Put(context.Background(), &Mention{
		Source: "https://stackoverflow.com/foo",
		Target: "https://bitworking.org/bar",
		State:  GOOD_STATE,
		TS:     time.Now(),
	})
	assert.NoError(t, err)

	err = m.Put(context.Background(), &Mention{
		Source: "https://spam.com/foo",
		Target: "https://bitworking.org/bar",
		State:  SPAM_STATE,
		TS:     time.Now(),
	})
	assert.NoError(t, err)

	err = m.Put(context.Background(), &Mention{
		Source: "https://news.ycombinator.com/foo",
		Target: "https://bitworking.org/bar",
		State:  GOOD_STATE,
		TS:     time.Now(),
	})
	assert.NoError(t, err)
	time.Sleep(2)

	mentions := m.GetGood(context.Background(), "https://bitworking.org/bar")
	assert.Len(t, mentions, 2)
}

func TestParseMicroformats(t *testing.T) {
	raw := `<article class="post h-entry" itemscope="" itemtype="http://schema.org/BlogPosting">

	<header class="post-header">
	<h1 class="post-title p-name" itemprop="name headline">WebMention Only</h1>
	<p class="post-meta">
	<a class="u-url" href="/news/2018/01/webmention-only">
    	<time datetime="2018-01-13T00:00:00-05:00" itemprop="datePublished" class="dt-published"> Jan 13, 2018 </time>
	</a>
	•
	<a rel="author" class="p-author h-card" href="/about">
	    <span itemprop="author" itemscope="" itemtype="http://schema.org/Person">
	        <img class="u-photo" src="/images/joe2016.jpg" alt="" style="height: 16px; border-radius: 8px; margin-right: 4px;">
	        <span itemprop="name">Joe Gregorio</span>
			</span>
	  </a>
	</p>
	</header>

	<div class="post-content e-content" itemprop="articleBody">
	<p><a href="https://allinthehead.com/retro/378/implementing-webmentions">Drew McLellan has gone WebMention-only.</a></p>

	<p>It’s an interesting idea, though I will still probably build a comment system
	for this blog and replace Disqus.</p>

	</div>
	<div id="mentions"></div>
</article>`

	m := InitForTesting(t)

	reader := bytes.NewReader([]byte(raw))
	u, err := url.Parse("https://bitworking.org/news/2018/01/webmention-only")
	assert.NoError(t, err)
	data := microformats.Parse(reader, u)
	mention := &Mention{
		Source: "https://bitworking.org/news/2018/01/webmention-only",
	}
	urlToImageReader := func(url string) (io.ReadCloser, error) {
		return os.Open("./testdata/author_image.jpg")
	}
	m.findHEntry(context.Background(), urlToImageReader, mention, data, data.Items)
	assert.Equal(t, "Joe Gregorio", mention.Author)
	assert.Equal(t, "2018-01-13 00:00:00 -0500 EST", mention.Published.String())
	assert.Equal(t, "f3f799d1a61805b5ee2ccb5cf0aebafa", mention.Thumbnail)
	assert.Equal(t, "https://bitworking.org/about", mention.AuthorURL)
}
