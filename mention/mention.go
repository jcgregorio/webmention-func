package mention

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/json"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/iterator"
	"willnorris.com/go/microformats"
	"willnorris.com/go/webmention"

	"github.com/jcgregorio/webmention-func/atom"
	"github.com/jcgregorio/webmention-func/ds"
	"github.com/nfnt/resize"
)

const (
	MENTIONS         ds.Kind = "Mentions"
	WEB_MENTION_SENT ds.Kind = "WebMentionSent"
	THUMBNAIL        ds.Kind = "Thumbnail"
)

func close(c io.Closer) {
	if err := c.Close(); err != nil {
		fmt.Printf("Failed to close: %s", err)
	}
}

type Mentions struct {
	DS *ds.DS
}

func NewMentions(ctx context.Context, project, ns string) (*Mentions, error) {
	d, err := ds.New(ctx, project, ns)
	if err != nil {
		return nil, err
	}
	return &Mentions{
		DS: d,
	}, nil
}

type WebMentionSent struct {
	TS time.Time
}

func (m *Mentions) sent(source string) (time.Time, bool) {
	key := m.DS.NewKey(WEB_MENTION_SENT)
	key.Name = source

	dst := &WebMentionSent{}
	if err := m.DS.Client.Get(context.Background(), key, dst); err != nil {
		fmt.Printf("Failed to find source: %q", source)
		return time.Time{}, false
	} else {
		fmt.Printf("Found source: %q", source)
		return dst.TS, true
	}
}

func (m *Mentions) recordSent(source string, updated time.Time) error {
	key := m.DS.NewKey(WEB_MENTION_SENT)
	key.Name = source

	src := &WebMentionSent{
		TS: updated.UTC(),
	}
	_, err := m.DS.Client.Put(context.Background(), key, src)
	return err
}

func (m *Mentions) ProcessAtomFeed(c *http.Client, filename string) error {
	fmt.Printf("Processing Atom Feed")
	f, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer close(f)
	mentionSources, err := ParseAtomFeed(f)
	if err != nil {
		return err
	}
	wmc := webmention.New(c)
	for source, ms := range mentionSources {
		ts, ok := m.sent(source)
		fmt.Printf("Updated: %v  ts: %v ok: %v after: %v", ms.Updated.Unix(), ts.Unix(), ok, ms.Updated.After(ts.Add(time.Second)))
		if ok && ts.Before(ms.Updated.Add(time.Second)) {
			fmt.Printf("Skipping since already sent: %s", source)
			continue
		}
		fmt.Printf("Processing Source: %s", source)
		for _, target := range ms.Targets {
			fmt.Printf("  to Target: %s", target)
			endpoint, err := wmc.DiscoverEndpoint(target)
			if err != nil {
				fmt.Printf("Failed looking for endpoint: %s", err)
				continue
			} else if endpoint == "" {
				fmt.Printf("No webmention support at: %s", target)
				continue
			}
			_, err = wmc.SendWebmention(endpoint, source, target)
			if err != nil {
				fmt.Printf("Error sending webmention to %s: %s", target, err)
			} else {
				fmt.Printf("Sent webmention from %s to %s", source, target)
			}
		}
		if err := m.recordSent(source, ms.Updated); err != nil {
			fmt.Printf("Failed recording Sent state: %s", err)
		}
	}
	return nil
}

type MentionSource struct {
	Targets []string
	Updated time.Time
}

func ParseAtomFeed(r io.Reader) (map[string]*MentionSource, error) {
	ret := map[string]*MentionSource{}
	b, err := ioutil.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to read feed: %s", err)
	}
	feed, err := atom.Parse(b)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse feed: %s", err)
	}
	for _, entry := range feed.Entry {
		buf := bytes.NewBufferString(entry.Content)
		links, err := webmention.DiscoverLinksFromReader(buf, entry.Link.HREF, "")
		if err != nil {
			fmt.Printf("Failed while discovering links in %q: %s", entry.Link.HREF, err)
			continue
		}
		updated, err := time.Parse(time.RFC3339, entry.Updated)
		if err != nil {
			fmt.Errorf("Failed to parse entry timestamp: %s", err)
		}
		ret[entry.Link.HREF] = &MentionSource{
			Targets: links,
			Updated: updated,
		}
	}
	return ret, nil
}

const (
	GOOD_STATE      = "good"
	UNTRIAGED_STATE = "untriaged"
	SPAM_STATE      = "spam"
)

type Mention struct {
	Source string
	Target string
	State  string
	TS     time.Time

	// Metadata found when validating. We might display this.
	Title     string    `datastore:",noindex"`
	Author    string    `datastore:",noindex"`
	AuthorURL string    `datastore:",noindex"`
	Published time.Time `datastore:",noindex"`
	Thumbnail string    `datastore:",noindex"`
}

func New(source, target string) *Mention {
	return &Mention{
		Source: source,
		Target: target,
		State:  UNTRIAGED_STATE,
		TS:     time.Now(),
	}
}

func (m *Mention) key() string {
	return fmt.Sprintf("%x", md5.Sum([]byte(m.Source+m.Target)))
}

func (m *Mention) FastValidate() error {
	if m.Source == "" {
		return fmt.Errorf("Source is empty.")
	}
	if m.Target == "" {
		return fmt.Errorf("Target is empty.")
	}
	if m.Target == m.Source {
		return fmt.Errorf("Source and Target must be different.")
	}
	target, err := url.Parse(m.Target)
	if err != nil {
		return fmt.Errorf("Target is not a valid URL: %s", err)
	}
	if target.Hostname() != "bitworking.org" {
		return fmt.Errorf("Wrong target domain.")
	}
	if target.Scheme != "https" {
		return fmt.Errorf("Wrong scheme for target.")
	}
	return nil
}

func (m *Mentions) SlowValidate(mention *Mention, c *http.Client) error {
	fmt.Printf("SlowValidate: %q", mention.Source)
	resp, err := c.Get(mention.Source)
	if err != nil {
		return fmt.Errorf("Failed to retrieve source: %s", err)
	}
	defer close(resp.Body)
	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to read content: %s", err)
	}
	reader := bytes.NewReader(b)
	links, err := webmention.DiscoverLinksFromReader(reader, mention.Source, "")
	if err != nil {
		return fmt.Errorf("Failed to discover links: %s", err)
	}
	for _, link := range links {
		if link == mention.Target {
			_, err := reader.Seek(0, io.SeekStart)
			if err != nil {
				return nil
			}
			m.ParseMicroformats(mention, reader, MakeUrlToImageReader(c))
			return nil
		}
	}
	return fmt.Errorf("Failed to find target link in source.")
}

func (m *Mentions) ParseMicroformats(mention *Mention, r io.Reader, urlToImageReader UrlToImageReader) {
	u, err := url.Parse(mention.Source)
	if err != nil {
		return
	}
	data := microformats.Parse(r, u)
	b, err := json.MarshalIndent(data, "", "  ")
	if err == nil {
		fmt.Printf("JSON: %q\n", string(b))
	} else {
		fmt.Printf("Errors parsing microformats: %s", err)
	}
	m.findHEntry(context.Background(), urlToImageReader, mention, data, data.Items)
	// Find an h-entry with the m.Target.
}

func (m *Mentions) VerifyQueuedMentions(c *http.Client) {
	queued := m.GetQueued(context.Background())
	fmt.Printf("About to slow verify %d queud mentions.", len(queued))
	for _, mention := range queued {
		fmt.Printf("Verifying queued webmention from %q", mention.Source)
		if m.SlowValidate(mention, c) == nil {
			mention.State = GOOD_STATE
		} else {
			mention.State = SPAM_STATE
			fmt.Printf("Failed to validate webmention: %#v", *mention)
		}
		if err := m.Put(context.Background(), mention); err != nil {
			fmt.Printf("Failed to save validated message: %s", err)
		}
	}
}

func (m *Mentions) get(ctx context.Context, target string, all bool) []*Mention {
	ret := []*Mention{}
	q := m.DS.NewQuery(MENTIONS).
		Filter("Target =", target)
	if !all {
		q = q.Filter("State =", GOOD_STATE)
	}

	it := m.DS.Client.Run(ctx, q)
	for {
		m := &Mention{}
		_, err := it.Next(m)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("Failed while reading: %s", err)
			break
		}
		ret = append(ret, m)
	}
	return ret
}

func (m *Mentions) GetAll(ctx context.Context, target string) []*Mention {
	return m.get(ctx, target, true)
}

func (m *Mentions) GetGood(ctx context.Context, target string) []*Mention {
	return m.get(ctx, target, false)
}

func (m *Mentions) UpdateState(ctx context.Context, encodedKey, state string) error {
	tx, err := m.DS.Client.NewTransaction(ctx)
	if err != nil {
		return fmt.Errorf("client.NewTransaction: %v", err)
	}
	key, err := datastore.DecodeKey(encodedKey)
	if err != nil {
		return fmt.Errorf("Unable to decode key: %s", err)
	}
	var mention Mention
	if err := tx.Get(key, &mention); err != nil {
		tx.Rollback()
		return fmt.Errorf("tx.GetMulti: %v", err)
	}
	mention.State = state
	if _, err := tx.Put(key, &mention); err != nil {
		tx.Rollback()
		return fmt.Errorf("tx.Put: %v", err)
	}
	if _, err = tx.Commit(); err != nil {
		return fmt.Errorf("tx.Commit: %v", err)
	}
	return nil
}

type MentionWithKey struct {
	Mention
	Key string
}

func (m *Mentions) GetTriage(ctx context.Context, limit, offset int) []*MentionWithKey {
	ret := []*MentionWithKey{}
	q := m.DS.NewQuery(MENTIONS).Order("-TS").Limit(limit).Offset(offset)

	it := m.DS.Client.Run(ctx, q)
	for {
		var m Mention
		key, err := it.Next(&m)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("Failed while reading: %s", err)
			break
		}
		ret = append(ret, &MentionWithKey{
			Mention: m,
			Key:     key.Encode(),
		})
	}
	return ret
}

func (m *Mentions) GetQueued(ctx context.Context) []*Mention {
	ret := []*Mention{}
	q := m.DS.NewQuery(MENTIONS).
		Filter("State =", UNTRIAGED_STATE)

	it := m.DS.Client.Run(ctx, q)
	for {
		m := &Mention{}
		_, err := it.Next(m)
		if err == iterator.Done {
			break
		}
		if err != nil {
			fmt.Printf("Failed while reading: %s", err)
			break
		}
		ret = append(ret, m)
	}
	return ret
}

func (m *Mentions) Put(ctx context.Context, mention *Mention) error {
	// TODO See if there's an existing mention already, so we don't overwrite its status?
	key := m.DS.NewKey(MENTIONS)
	key.Name = mention.key()
	if _, err := m.DS.Client.Put(ctx, key, mention); err != nil {
		return fmt.Errorf("Failed writing %#v: %s", *mention, err)
	}
	return nil
}

type UrlToImageReader func(url string) (io.ReadCloser, error)

func in(s string, arr []string) bool {
	for _, a := range arr {
		if a == s {
			return true
		}
	}
	return false
}

func firstPropAsString(uf *microformats.Microformat, key string) string {
	fmt.Printf("firstPropAsString: %s %s", uf.Properties, key)
	for _, sint := range uf.Properties[key] {
		if s, ok := sint.(string); ok {
			return s
		}
	}
	return ""
}

func (m *Mentions) findHEntry(ctx context.Context, u2r UrlToImageReader, mention *Mention, data *microformats.Data, items []*microformats.Microformat) {
	for _, it := range items {
		if in("h-entry", it.Type) {
			mention.Title = firstPropAsString(it, "name")
			if strings.HasPrefix(mention.Title, "tag:twitter") {
				mention.Title = "Twitter"
				if firstPropAsString(it, "like-of") != "" {
					mention.Title += " Like"
				}
				if firstPropAsString(it, "repost-of") != "" {
					mention.Title += " Repost"
				}
			}
			if t, err := time.Parse(time.RFC3339, firstPropAsString(it, "published")); err == nil {
				mention.Published = t
			}
			if authorsInt, ok := it.Properties["author"]; ok {
				for _, authorInt := range authorsInt {
					if author, ok := authorInt.(*microformats.Microformat); ok {
						m.findAuthor(ctx, u2r, mention, data, author)
					}
				}
			}
		}
		m.findHEntry(ctx, u2r, mention, data, it.Children)
	}
}

type Thumbnail struct {
	PNG []byte `datastore:",noindex"`
}

func MakeUrlToImageReader(c *http.Client) UrlToImageReader {
	return func(u string) (io.ReadCloser, error) {
		resp, err := c.Get(u)
		if err != nil {
			return nil, fmt.Errorf("Error retrieving thumbnail: %s", err)
		}
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Not a 200 response: %d", resp.StatusCode)
		}
		return resp.Body, nil
	}
}

func (m *Mentions) findAuthor(ctx context.Context, u2r UrlToImageReader, mention *Mention, data *microformats.Data, it *microformats.Microformat) {
	fmt.Printf("Found author in microformat.")
	mention.Author = it.Value
	mention.AuthorURL = data.Rels["author"][0]
	u := firstPropAsString(it, "photo")
	if u == "" {
		fmt.Printf("No photo URL found.")
		return
	}

	r, err := u2r(u)
	if err != nil {
		fmt.Printf("Failed to retrieve photo.")
		return
	}

	defer close(r)
	img, _, err := image.Decode(r)
	if err != nil {
		fmt.Printf("Failed to decode photo.")
		return
	}
	rect := img.Bounds()
	var x uint = 32
	var y uint = 32
	if rect.Max.X > rect.Max.Y {
		y = 0
	} else {
		x = 0
	}
	resized := resize.Resize(x, y, img, resize.Lanczos3)

	var buf bytes.Buffer
	encoder := png.Encoder{
		CompressionLevel: png.BestCompression,
	}
	if err := encoder.Encode(&buf, resized); err != nil {
		fmt.Printf("Failed to encode photo.")
		return
	}

	hash := fmt.Sprintf("%x", md5.Sum(buf.Bytes()))
	t := &Thumbnail{
		PNG: buf.Bytes(),
	}
	key := m.DS.NewKey(THUMBNAIL)
	key.Name = hash
	if _, err := m.DS.Client.Put(ctx, key, t); err != nil {
		fmt.Printf("Failed to write: %s", err)
		return
	}
	mention.Thumbnail = hash
}

func (m *Mentions) GetThumbnail(ctx context.Context, id string) ([]byte, error) {
	key := m.DS.NewKey(THUMBNAIL)
	key.Name = id
	var t Thumbnail
	if err := m.DS.Client.Get(ctx, key, &t); err != nil {
		return nil, fmt.Errorf("Failed to find image: %s", err)
	}
	return t.PNG, nil

}
