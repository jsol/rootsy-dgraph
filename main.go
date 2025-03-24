package main

import (
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"

	dgo "github.com/dgraph-io/dgo/v230"
	"github.com/dgraph-io/dgo/v230/protos/api"

	"github.com/fsnotify/fsnotify"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type application struct {
	auth struct {
		username [32]byte
		password [32]byte
	}
	spotify_cred struct {
		key    string
		secret string
	}
	port         int
	sp           *Spotify
	conn         *grpc.ClientConn
	templates    *template.Template
	debug        bool
	StaticPath   string
	TemplatePath string
}

type DGraphLink struct {
	Id   string `json:"uid,omitempty"`
	Text string `json:"text"`
	Href string `json:"href"`
}

type DGraphArtist struct {
	Id           int64           `json:"oldId"`
	Uid          string          `json:"uid,omitempty"`
	Name         string          `json:"name"`
	Presentation string          `json:"text"`
	Pic          string          `json:"pic"`
	Pictext      string          `json:"picText"`
	Content      []DGraphContent `json:"content"`
	NumContent   int             `json:"num_content"`
	Link         []DGraphLink    `json:"link"`
	DType        string          `json:"dgraph.type,omitempty"`
}

type DGraphContributor struct {
	Id           int64           `json:"oldId"`
	Uid          string          `json:"uid,omitempty"`
	Name         string          `json:"name"`
	Presentation string          `json:"text"`
	Pic          string          `json:"pic"`
	Pictext      string          `json:"picText"`
	DType        string          `json:"dgraph.type,omitempty"`
	Content      []DGraphContent `json:"content"`
}

type DGraphLabel struct {
	Uid     string          `json:"uid,omitempty"`
	Name    string          `json:"name"`
	DType   string          `json:"dgraph.type,omitempty"`
	Content []DGraphContent `json:"content"`
}

type DGraphContent struct {
	Id          string              `json:"oldId"`
	Uid         string              `json:"uid,omitempty"`
	Name        string              `json:"name"`
	Text        string              `json:"text"`
	LeadInText  string              `json:"lead_in_text"`
	Label       []DGraphLabel       `json:"label"`
	CreatedAt   string              `json:"created_at"`
	PublishedAt string              `json:"published_at"`
	WrittenBy   []DGraphContributor `json:"written_by"`
	Artist      []DGraphArtist      `json:"artist"`
	Spotify     string              `json:"spotify"`
	Pic         string              `json:"pic"`
	Pictext     string              `json:"picText"`
	ViewCount   int                 `json:"view_count"`
	Type        string              `json:"type"`
	DType       string              `json:"dgraph.type,omitempty"`
	Content     []DGraphContent
}

type DGraphStats struct {
	Uid       string              `json:"uid,omitempty"`
	Name      string              `json:"name"`
	WrittenBy []DGraphContributor `json:"written_by"`
	Artist    []DGraphArtist      `json:"artist"`
	Pic       string              `json:"pic"`
	ReadCount int                 `json:"read_count"`
	ViewCount int                 `json:"view_count"`
	Type      string              `json:"type"`
	DType     string              `json:"dgraph.type,omitempty"`
}

type Counter struct {
	Name      string `json:"name"`
	Uid       string `json:"uid"`
	ReadCount int64  `json:"read_count"`
}

type Random struct {
	Uid       string `json:"uid"`
	Random    int    `json:"random"`
	ViewCount int    `json:"view_count"`
}

type StatsResponse struct {
	Stats []DGraphStats `json:"content"`
}

type CounterResponse struct {
	Counter []Counter `json:"counter"`
}

type ArtistResponse struct {
	Artist []DGraphArtist `json:"artist"`
}

type ContentResponse struct {
	Content []DGraphContent `json:"content"`
	Extra   []DGraphContent `json:"extra"`
	Rootsy  []DGraphLabel   `json:"rootsy"`
}
type ExtraContent struct {
	Uid        string `json:"uid,omitempty"`
	Name       string `json:"name"`
	Url        string `json:"url"`
	LeadInText string `json:"lead_in_text"`
	Pic        string `json:"pic"`
	Type       string `json:"type"`
	TypeText   string `json:"type_text"`
	WrittenBy  string `json:"written_by"`
}

type UpdateSpotify struct {
	Uid     string `json:"uid"`
	Spotify string `json:"spotify"`
}

type PrintSpotify struct {
	Name    string
	Artist  string
	Id      string
	OldId   string
	Image   string
	Options []SpotifyOption
}

func clearMarkers(input string) string {
	input = regexp.MustCompile(`\[.*\]`).ReplaceAllString(input, "")
	input = regexp.MustCompile(`\"(.*?)\"`).ReplaceAllString(input, "»$1«")
	input = regexp.MustCompile(`”(.*?)”`).ReplaceAllString(input, "»$1«")
	input = regexp.MustCompile(`<94>(.*?)<94>`).ReplaceAllString(input, "»$1«")

	return input
}

func escapeText(input string) template.HTML {
	input = regexp.MustCompile(`\"(.*?)\"`).ReplaceAllString(input, "»$1«")
	input = regexp.MustCompile(`”(.*?)”`).ReplaceAllString(input, "»$1«")
	input = regexp.MustCompile(`<94>(.*?)<94>`).ReplaceAllString(input, "»$1«")
	input = template.HTMLEscapeString(input)
	input = regexp.MustCompile(`\n`).ReplaceAllString(input, "<br>")
	input = regexp.MustCompile(`\[b\](.*?)\[/b\]`).ReplaceAllString(input, "<b>$1</b>")
	input = regexp.MustCompile(`\[i\](.*?)\[/i\]`).ReplaceAllString(input, "<i>$1</i>")
	input = regexp.MustCompile(`\[url http://youtu.be/(.*?)\](.*?)\[/url\]`).ReplaceAllString(input, "<br clear='all'><center><iframe width='420' height='315' src='http://www.youtube.com/embed/$1?rel=0' frameborder='0' allowfullscreen></iframe></center>")
	input = regexp.MustCompile(`\[http://www.rootsy.nu/(.*?)\]`).ReplaceAllString(input, "<a href='$1'>www.rootsy.nu/$1</a>")
	input = regexp.MustCompile(`\[http://(.*?)\]`).ReplaceAllString(input, "<a href='http://$1' target='_blank'>$1</a>")
	input = regexp.MustCompile(`\[url http://www.rootsy.nu/(.*?)\](.*?)\[/url\]`).ReplaceAllString(input, "<a href='http://www.rootsy.nu/$1'>$2</a>")
	input = regexp.MustCompile(`\[url (.*?)\](.*?)\[/url\]`).ReplaceAllString(input, "<a href='$1' target='_blank'>$2</a>")
	input = regexp.MustCompile(`\[img ([^ ]*?)\]`).ReplaceAllString(input, "<img src='http://files.rootsy.nu/rpb/extra/$1'>")
	input = regexp.MustCompile(`\[img ([^ ]*?) (.*?)]`).ReplaceAllString(input, "<img src='http://files.rootsy.nu/rpb/extra/$1' TITLE='$2'>")
	/**


		v $replacements[] = "<br clear='all'><center><iframe width='420' height='315' src='http://www.youtube.com/embed/$1?rel=0' frameborder='0' allowfullscreen></iframe></center>";
	        $replacements[] = "<a href='\$1'>www.rootsy.nu/\$1</a>";
	        $replacements[] = "<a href='http://\$1' target='_blank'>\$1</a>";
	        $replacements[] = "<a href='http://www.rootsy.nu/\$1'>\$2</a>";
	        $replacements[] = "<a href='\$1' target='_blank'>\$2</a>";
	        $replacements[] = "<img src='http://www.rootsy.nu/bilder/extra/\$1' id='bild2'>";
	        $replacements[] = "<img src='http://www.rootsy.nu/bilder/extra/\$1' id='bild2' TITLE='$2'>";
	        $replacements[] = "»$1«";
	        $replacements[] = "»$1«";

			       $patterns[] = "|\[url http://youtu.be/(.*?)\](.*?)\[/url\]|s";
		        $patterns[] = "|\[http://www.rootsy.nu/(.*?)\]|s";
		        $patterns[] = "|\[http://(.*?)\]|s";
		        $patterns[] = "|\[url http://www.rootsy.nu/(.*?)\](.*?)\[/url\]|s";
		        $patterns[] = "|\[url (.*?)\](.*?)\[/url\]|s";
		        $patterns[] = "|\[img ([^ ]*?)\]|s";
		        $patterns[] = "|\[img ([^ ]*?) (.*?)]|s";
		        $patterns[] = "|\"(.*?)\"|s";
		        $patterns[] = "|<94>(.*?)<94>|s";

	*/
	return template.HTML(input)
}

func artistNames(list []DGraphArtist) string {

	var names []string

	for _, a := range list {
		names = append(names, a.Name)
	}

	return strings.Join(names, " & ")
}
func toUrl(name string) string {
	return regexp.MustCompile(`[^a-zA-Z0-9]+`).ReplaceAllString(name, "")
}

func typeText(name string) string {
	switch name {
	case "review":
		return "Recension"
	case "article":
		return "Artikel"
	case "chart":
		return "Toplista"
	case "pitch":
		return "Tips"
	default:
		return name
	}
}

func (app *application) printStart(wr io.Writer, ctx context.Context) {

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query Content {
	rootsy(func:eq(name, "rootsy")){
	  uid
	  name
	  content:~label ( orderasc:read_count, orderasc:random, first: 5){
		  name
			  lead_in_text
			  type
			  uid
			  pic
			  published_at
			  artist {
				  name
			  }
			  written_by {
				  name
			  }
	}
	}
			  extra(func:has(read_count), orderasc:read_count, orderasc:random, first: 15) @filter(type(Content)) {
			  name
			  lead_in_text
			  type
			  uid
			  pic
			  published_at
			  artist {
				  name
			  }
			  written_by {
				  name
			  }
			}
		}`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.Query(ctx, q)
	if err != nil {

		panic(err.Error())
	}

	var resp ContentResponse

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	startContent := []DGraphContent{}
	pickContent("", &startContent, resp.Extra, 16)
	pickContent("", &startContent, resp.Rootsy[0].Content, 24)
	updateRandom(startContent, dg, ctx)

	rand.Shuffle(len(startContent), func(i, j int) {
		startContent[i], startContent[j] = startContent[j], startContent[i]
	})

	app.executeTemplate(wr, "start", startContent)
}

func (app *application) printSearch(search string, wr io.Writer, ctx context.Context) {

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query Search ($terms: string){
		Content(func:allofterms(name, $terms), first: 20)  @filter(type(Content)){
		  	name
			lead_in_text
			type
			uid
			pic
			published_at
			artist {
				name
			}
			written_by {
				name
			}
		}
	}`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.QueryWithVars(ctx, q, map[string]string{"$terms": search})
	if err != nil {

		panic(err.Error())
	}

	var resp ContentResponse

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	app.executeTemplate(wr, "search", resp.Content)
}

func (app *application) executeTemplate(wr io.Writer, name string, content any) {

	err := app.templates.ExecuteTemplate(wr, name, content)
	if err != nil {
		fmt.Println(err)
	}
}

func (app *application) apiExtraContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query Content {
		extra(func:has(read_count), orderasc:read_count,orderasc:view_count, orderasc:random, first: 15) @filter(type(Content)) {
			name
			lead_in_text
			type
			uid
			pic
			published_at
			artist {
				name
			}
			written_by {
				name
			}
	  	}
	  }`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.Query(ctx, q)
	if err != nil {

		panic(err.Error())
	}

	var resp ContentResponse

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	updateRandom(resp.Extra, dg, ctx)
	w.Header().Set("Content-Type", "application/json")
	list := []ExtraContent{}
	for _, c := range resp.Extra {
		list = append(list, ExtraContent{
			Name:       c.Name,
			Uid:        c.Uid,
			Url:        fmt.Sprintf("/content/%s/%s", c.Uid, toUrl(c.Name)),
			LeadInText: string(clearMarkers(c.LeadInText)),
			Pic:        c.Pic,
			Type:       c.Type,
			TypeText:   typeText(c.Type),
			WrittenBy:  c.WrittenBy[0].Name,
		})
	}

	pb, err := json.Marshal(list)
	w.Write(pb)
}

func (app *application) printContent(uid string, wr io.Writer, ctx context.Context) {

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query Content($terms: string) {
		extra(func:has(read_count), orderasc:read_count, orderasc:random, first: 15) @filter(type(Content)) {
			name
			lead_in_text
			type
			uid
			pic
			published_at
			artist {
				name
			}
			written_by {
				name
			}
	  	}
		content(func: uid($terms)) {
			uid
		   	name
		   	text
		  	type
		  	pic
		  	published_at
		  	spotify
		  	artist{
				name
				uid
				pic
				num_content: count(~artist)
				content: ~artist  (first:10, orderasc:read_count, orderasc:random){
					name
					lead_in_text
					type
					uid
					pic
					published_at
					artist {
						name
					}
					written_by {
						name
					}
				}
			}
			written_by {
				name
				content: ~written_by (first:10, orderasc:read_count, orderasc:random){
					name
					lead_in_text
					type
					uid
					pic
					published_at
					artist {
						name
					}
					written_by {
						name
					}
				 }
			}
			label {
				name
				content: ~label (first:10, orderasc:read_count,  orderasc:random){
					name
					lead_in_text
					type
					uid
					pic
					published_at
					artist {
						name
					}
					written_by {
						name
					}
				}
			}
		}
	  }`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.QueryWithVars(ctx, q, map[string]string{"$terms": uid})
	if err != nil {

		panic(err.Error())
	}

	var resp ContentResponse

	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	c := resp.Content[0]
	for _, l := range c.Label {
		pickContent(c.Uid, &c.Content, l.Content, 6)
		for _, c2 := range l.Content {

			fmt.Println(c2.Name)
		}
	}

	for _, l := range c.Artist {
		pickContent(c.Uid, &c.Content, l.Content, 10)
		for _, c2 := range l.Content {

			fmt.Println(c2.Name)
		}

	}
	for _, l := range c.WrittenBy {
		for _, c2 := range l.Content {

			fmt.Println(c2.Name)
		}
		pickContent(c.Uid, &c.Content, l.Content, 14)
	}

	for _, c2 := range resp.Extra {
		fmt.Println(c2.Name)
	}

	pickContent(c.Uid, &c.Content, resp.Extra, 16)

	rand.Shuffle(len(c.Content), func(i, j int) {
		c.Content[i], c.Content[j] = c.Content[j], c.Content[i]
	})

	updateRandom(c.Content, dg, ctx)

	for _, c2 := range c.Content {

		fmt.Println(c2.Name)
	}

	if c.Spotify == "x" {
		c.Spotify = ""
	}

	app.executeTemplate(wr, "content", c)
}

func (app *application) printArtist(uid string, wr io.Writer, ctx context.Context) {

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query Artist($terms: string) {
		artist(func: uid($terms)) @filter(type(Artist)) {
			uid
		   	name
		   	text
		  	pic
			link {
				text
				href
			}
			  content: ~artist{
				name
			   	lead_in_text
			   	type
			   	uid
			   	pic
			   	published_at
			   	artist {
				   name
			    }
				written_by {
				   name
			    }
		   }
		}
	  }`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.QueryWithVars(ctx, q, map[string]string{"$terms": uid})
	if err != nil {

		panic(err.Error())
	}

	var resp ArtistResponse

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	app.executeTemplate(wr, "artist", resp.Artist[0])

}

func hasContent(haystack *[]DGraphContent, needle string) bool {
	for _, c := range *haystack {
		if c.Uid == needle {
			return true
		}
	}
	return false
}

func pickContent(current string, existing *[]DGraphContent, candidates []DGraphContent, wanted int) {

	num := wanted - len(*existing)
	for i := 0; i < num && i < len(candidates); i++ {

		if candidates[i].Uid == current {
			continue
		}
		if hasContent(existing, candidates[i].Uid) {
			continue
		}
		*existing = append(*existing, candidates[i])
	}
}

func (app *application) handler(w http.ResponseWriter, r *http.Request) {

	parts := strings.SplitN(r.URL.Path, "/", 4)
	if len(parts) >= 3 {
		switch parts[1] {
		case "artist":
			app.printArtist(parts[2], w, r.Context())
		case "content":
			app.printContent(parts[2], w, r.Context())
		case "search":
			app.printSearch(strings.Join(r.URL.Query()["terms"], " "), w, r.Context())
		default:
			app.printStart(w, r.Context())
		}
	} else {
		app.printStart(w, r.Context())
	}

	if app.debug {
		script := `<script>
	const refresher = new EventSource("/sse")
	refresher.onmessage = (event) => {
		console.log("Event handler called!", event.data)
		if (event.data == "reload") {
			location.reload()
		}
	}
	</script>`
		w.Write([]byte(script))
	}
}

func (app *application) readCounter(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.Path, "/", 5)
	if len(parts) < 4 {
		return
	}
	fmt.Printf("Read: %s", parts[2])
	app.updateCounter(parts[2], parts[3], r.Context())
}

func (app *application) sse(w http.ResponseWriter, r *http.Request) {

	fmt.Println("new SSE")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.WriteHeader(200)

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}

	defer watcher.Close()

	err = watcher.Add(app.TemplatePath)
	if err != nil {
		panic(err)
	}
	err = watcher.Add(app.StaticPath)
	if err != nil {
		panic(err)
	}
	w.Write([]byte("data: hello\n\n"))
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}
	for {
		select {
		case ev, ok := <-watcher.Events:
			if !ok {
				fmt.Println("Exit watcher 1")
				return
			}
			if ev.Has(fsnotify.Write) {
				tmp, err := template.New("rootsy").Funcs(template.FuncMap{"escapeText": escapeText, "clearMarkers": clearMarkers, "artistsNames": artistNames, "toUrl": toUrl, "typeText": typeText}).ParseGlob(app.TemplatePath + "/*.tmpl")
				//t, err := template.ParseGlob("templates/*.tmpl")

				if err != nil {
					fmt.Println(err)
				} else {
					app.templates = tmp
					w.Write([]byte("data: reload\n\n"))
					if f, ok := w.(http.Flusher); ok {
						f.Flush()
					}
					fmt.Println("Got event, sending...")
				}

			}
		case err, ok := <-watcher.Errors:
			if !ok {
				fmt.Println("Exit watcher 2")
				return
			}
			panic(err)
		case <-r.Context().Done():
			fmt.Println("Exit watcher 3")
			return
		}

	}

}

func updateRandom(content []DGraphContent, dg *dgo.Dgraph, ctx context.Context) {
	update := []Random{}

	for _, c := range content {
		r := Random{}
		r.Uid = c.Uid
		r.Random = rand.Intn(1000)
		r.ViewCount = c.ViewCount + 1
		update = append(update, r)
	}

	pb, err := json.Marshal(update)
	// Check error
	if err != nil {
		panic(err.Error())
	}

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetJson:   pb,
		CommitNow: true,
	}

	_, err = txn.Mutate(ctx, mu)
	if err != nil {
		fmt.Println(err)
	}
}

func (app *application) updateCounter(uid, uuid string, ctx context.Context) {

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query CounterQuery($terms: string) {
		counter (func: uid($terms)) @filter(type(Content)) {
			uid
			name
		  	read_count
		}
	  }`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.QueryWithVars(ctx, q, map[string]string{"$terms": uid})
	if err != nil {

		panic(err.Error())
	}

	var resp CounterResponse

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	fmt.Println(resp)
	if len(resp.Counter) != 1 {
		return
	}

	item := resp.Counter[0]

	item.ReadCount++

	pb, err := json.Marshal(item)
	// Check error
	if err != nil {
		panic(err.Error())
	}

	mu := &api.Mutation{
		SetJson:   pb,
		CommitNow: true,
	}

	_, err = txn.Mutate(ctx, mu)
	if err != nil {
		fmt.Println(err)
	}

	txn = dg.NewTxn()
	defer txn.Discard(ctx)
	ts := time.Now().Format(time.RFC3339)

	fmt.Println(ts)

	q = `query Viewer($terms: string){
		Q(func:eq(uuid, $terms)) {
			v as uid
		}
	}`
	mu = &api.Mutation{
		SetNquads: []byte(`uid(v) <uuid> "` + uuid + `" .
		uid(v) <dgraph.type> "Viewer" .
		uid(v) <content> <` + uid + `> (time=` + ts + `) .`),
	}

	fmt.Println(`viewer <uuid> "` + uuid + `" .
	viewer <dgraph.type> "Viewer" .
	viewer <content> "` + uid + `" (time=` + ts + `) .`)
	req := &api.Request{
		Query:     q,
		Mutations: []*api.Mutation{mu},
		Vars:      map[string]string{"$terms": uuid},
		CommitNow: true,
	}
	_, err = txn.Do(ctx, req)
	if err != nil {
		fmt.Println(err)
	}
}

func UpdateSpotifyUrl(uid, url string, dg *dgo.Dgraph, ctx context.Context) {
	update := UpdateSpotify{
		Uid:     uid,
		Spotify: url,
	}
	pb, err := json.Marshal(update)
	// Check error
	if err != nil {
		panic(err.Error())
	}

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	mu := &api.Mutation{
		SetJson:   pb,
		CommitNow: true,
	}

	_, err = txn.Mutate(ctx, mu)
	if err != nil {
		fmt.Println(err)
	}
}

func saveSpotify(oldid, url string) {
	f, err := os.OpenFile("spotify.tab",
		os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Println(err)
	}
	defer f.Close()
	if _, err := f.WriteString(fmt.Sprintf("%s\t%s\n", oldid, url)); err != nil {
		log.Println(err)
	}
}

func (app *application) spotify(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	id := r.PostFormValue("id")

	if id != "" {
		saveSpotify(r.PostFormValue("oldid"), r.PostFormValue("url"))
		UpdateSpotifyUrl(id, r.PostFormValue("url"), dg, ctx)
	}

	q := `query SpotifyQuery {
		content (func: eq(spotify, "x"), first: 1) @filter(type(Content) AND NOT eq(type, "article")) {
			uid
			oldId
			pic
			name: album
      		artist {
        		name
      		}
		}
	  }`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.Query(ctx, q)
	if err != nil {
		panic(err.Error())
	}

	var resp ContentResponse

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	fmt.Println(resp)
	if len(resp.Content) != 1 {
		return
	}

	artistName := "No artist"
	if len(resp.Content) > 0 && len(resp.Content[0].Artist) > 0 {
		artistName = resp.Content[0].Artist[0].Name
	}
	album := resp.Content[0].Name

	opts, err := app.sp.Search(strings.TrimSuffix(strings.TrimSuffix(artistName, ", The"), ", the"), album)

	if err != nil {
		fmt.Printf("Error searching Spotify: %v", err)
	}
	item := PrintSpotify{
		Name:    album,
		Artist:  artistName,
		Id:      resp.Content[0].Uid,
		OldId:   resp.Content[0].Id,
		Image:   resp.Content[0].Pic,
		Options: opts,
	}

	app.executeTemplate(w, "spotify", item)
}

func (app *application) stats(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query StatsQuery {
    		content (func: has(read_count), orderdesc:read_count, first: 50) @filter(type(Content)) {
        name
        pic
    read_count
    view_count
    written_by {
			name
    }
    artist{
	    name
    }
		}
	  }`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.Query(ctx, q)
	if err != nil {
		panic(err.Error())
	}

	var resp StatsResponse

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	app.executeTemplate(w, "stats", resp.Stats)
}

func (app *application) handleOldContent(prefix, cat string, w http.ResponseWriter, r *http.Request) {

	id := r.URL.Query()["id"]

	if len(id) != 1 {
		return
	}

	ctx := r.Context()

	dc := api.NewDgraphClient(app.conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query OldContent($terms: string) {
		content (func: eq(oldId, $terms)) {
			uid
			name
		}
	  }`

	txn := dg.NewTxn()
	defer txn.Discard(ctx)

	res, err := txn.QueryWithVars(ctx, q, map[string]string{"$terms": fmt.Sprintf("%s-%s", prefix, id[0])})
	if err != nil {

		panic(err.Error())
	}

	var resp DGraphContent

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	fmt.Println(resp)
	if len(resp.Content) != 1 {
		return
	}

	http.Redirect(w, r, fmt.Sprintf("https://www.rootsy.nu/%s/%s/%s", cat, resp.Content[0].Uid, toUrl(resp.Content[0].Name)), 301)
}

func (app *application) handleOldReview(w http.ResponseWriter, r *http.Request) {
	app.handleOldContent("r", "content", w, r)
}

func (app *application) handleOldArticle(w http.ResponseWriter, r *http.Request) {
	app.handleOldContent("f", "content", w, r)
}
func (app *application) handleOldArtist(w http.ResponseWriter, r *http.Request) {
	app.handleOldContent("a", "artist", w, r)
}

func (app *application) basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok {
			usernameHash := sha256.Sum256([]byte(username))
			passwordHash := sha256.Sum256([]byte(password))

			usernameMatch := (subtle.ConstantTimeCompare(usernameHash[:], app.auth.username[:]) == 1)
			passwordMatch := (subtle.ConstantTimeCompare(passwordHash[:], app.auth.password[:]) == 1)

			if usernameMatch && passwordMatch {
				next.ServeHTTP(w, r)
				return
			}
		}

		w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	})
}

func (app *application) favicon(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, app.StaticPath+"/favicon.ico")
}

func main() {
	app := new(application)
	app.port = 9090

	user := os.Getenv("AUTH_USERNAME")
	password := os.Getenv("AUTH_PASSWORD")
	root_path := os.Getenv("STATIC_PATH")
	// "127.0.0.1:9080"
	dgraph := os.Getenv("DGRAPH_URL")

	if user == "" {
		log.Fatal("basic auth username must be provided")
	}

	if password == "" {
		log.Fatal("basic auth password must be provided")
	}

	if dgraph == "" {
		log.Fatal("DGraph URL must be provided")
	}

	app.StaticPath = root_path + "static"
	app.TemplatePath = root_path + "templates"

	app.auth.username = sha256.Sum256([]byte(user))
	app.auth.password = sha256.Sum256([]byte(password))

	app.spotify_cred.key = os.Getenv("SPOTIFY_KEY")
	app.spotify_cred.secret = os.Getenv("SPOTIFY_SECRET")

	if app.spotify_cred.key == "" {
		log.Fatal("spotify key must be provided")
	}

	if app.spotify_cred.secret == "" {
		log.Fatal("spotify secret must be provided")
	}

	var err error
	app.sp, err = NewSpotify(app.spotify_cred.key, app.spotify_cred.secret)
	if err != nil {
		log.Fatalln("Error setting up spotify:", err)
	}

	if err != nil {
		panic(err)
	}

	if len(os.Args) == 2 && os.Args[1] == "spotify" {
		err = app.sp.ClearPlaylist("39kgihD6NPmYjAKM8wKCoM")
		if err != nil {
			panic(err)
		}
		err = app.sp.AddAlbumToPlaylist("39kgihD6NPmYjAKM8wKCoM", "34xaLN7rDecGEK5UGIVbeJ")
		if err != nil {
			panic(err)
		}
		return
	}

	app.conn, err = grpc.Dial(dgraph, grpc.WithTransportCredentials(insecure.NewCredentials()))

	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}

	app.templates, err = template.New("rootsy").Funcs(template.FuncMap{"escapeText": escapeText, "clearMarkers": clearMarkers, "artistsNames": artistNames, "toUrl": toUrl, "typeText": typeText}).ParseGlob(app.TemplatePath + "/*.tmpl")
	//t, err := template.ParseGlob("templates/*.tmpl")

	if err != nil {
		panic(err)
	}

	if app.debug {
		http.HandleFunc("/sse", app.sse)
	}

	http.HandleFunc("/favicon.ico", app.favicon)
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir(app.StaticPath))))
	http.HandleFunc("/read/", app.readCounter)
	http.HandleFunc("/spotify", app.basicAuth(app.spotify))
	http.HandleFunc("/stats", app.basicAuth(app.stats))
	http.HandleFunc("/api/content/extra", app.apiExtraContent)
	http.HandleFunc("/recension.php", app.handleOldReview)
	http.HandleFunc("/artikel.php", app.handleOldArticle)
	http.HandleFunc("/artist.php", app.handleOldArtist)

	http.HandleFunc("/", app.handler)                            // set router
	err = http.ListenAndServe(fmt.Sprintf(":%d", app.port), nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}
