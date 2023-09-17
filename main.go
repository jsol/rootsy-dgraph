package main

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"strings"
	"time"

	dgo "github.com/dgraph-io/dgo/v230"
	"github.com/dgraph-io/dgo/v230/protos/api"

	"github.com/fsnotify/fsnotify"

	_ "github.com/go-sql-driver/mysql"
	"google.golang.org/grpc"
)

type DGraphLink struct {
	Id string `json:"uid,omitempty"`
}

var port int

type DGraphArtist struct {
	Id           int64           `json:"oldId"`
	Uid          string          `json:"uid,omitempty"`
	Name         string          `json:"name"`
	Presentation string          `json:"text"`
	Pic          string          `json:"pic"`
	Pictext      string          `json:"picText"`
	Content      []DGraphContent `json:"content"`
	NumContent   int             `json:"num_content"`
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
	Id          int64               `json:"oldId"`
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

func escapeText(input string) template.HTML {
	input = template.HTMLEscapeString(input)
	input = regexp.MustCompile(`\n`).ReplaceAllString(input, "<br>")
	input = regexp.MustCompile(`\[b\](.*?)\[/b\]`).ReplaceAllString(input, "<b>$1</b>")
	input = regexp.MustCompile(`\[i\](.*?)\[/i\]`).ReplaceAllString(input, "<i>$1</i>")
	input = regexp.MustCompile(`\[url http://youtu.be/(.*?)\](.*?)\[/url\]`).ReplaceAllString(input, "<br clear='all'><center><iframe width='420' height='315' src='http://www.youtube.com/embed/$1?rel=0' frameborder='0' allowfullscreen></iframe></center>")
	input = regexp.MustCompile(`\[http://www.rootsy.nu/(.*?)\]`).ReplaceAllString(input, "<a href='$1'>www.rootsy.nu/$1</a>")
	input = regexp.MustCompile(`\[http://(.*?)\]`).ReplaceAllString(input, "<a href='http://$1' target='_blank'>$1</a>")
	input = regexp.MustCompile(`\[url http://www.rootsy.nu/(.*?)\](.*?)\[/url\]`).ReplaceAllString(input, "<a href='http://www.rootsy.nu/$1'>$2</a>")
	input = regexp.MustCompile(`\[url (.*?)\](.*?)\[/url\]`).ReplaceAllString(input, "<a href='$1' target='_blank'>$2</a>")
	input = regexp.MustCompile(`\[img ([^ ]*?)\]`).ReplaceAllString(input, "<img src='http://www.rootsy.nu/bilder/extra/$1'>")
	input = regexp.MustCompile(`\[img ([^ ]*?) (.*?)]`).ReplaceAllString(input, "<img src='http://www.rootsy.nu/bilder/extra/$1' TITLE='$2'>")
	input = regexp.MustCompile(`\"(.*?)\"`).ReplaceAllString(input, "»$1«")
	input = regexp.MustCompile(`<94>(.*?)<94>`).ReplaceAllString(input, "»$1«")
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

func printStart(wr io.Writer, ctx context.Context) {

	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
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

	executeTemplate(wr, "start", startContent)
}

func printSearch(search string, wr io.Writer, ctx context.Context) {

	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query Search ($terms: string){
		Content(func:allofterms(name, $terms))  @filter(type(Content)){
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

	executeTemplate(wr, "search", resp.Content)
}

func executeTemplate(wr io.Writer, name string, content any) {
	t, err := template.New("rootsy").Funcs(template.FuncMap{"escapeText": escapeText, "artistsNames": artistNames, "toUrl": toUrl, "typeText": typeText}).ParseGlob("templates/*.tmpl")
	//t, err := template.ParseGlob("templates/*.tmpl")

	if err != nil {
		panic(err)
	}

	err = t.ExecuteTemplate(wr, name, content)
	if err != nil {
		fmt.Println(err)
	}
}

func apiExtraContent(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
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
			LeadInText: string(escapeText(c.LeadInText)),
			Pic:        c.Pic,
			Type:       c.Type,
			TypeText:   typeText(c.Type),
			WrittenBy:  c.WrittenBy[0].Name,
		})
	}

	pb, err := json.Marshal(list)
	w.Write(pb)
}

func printContent(uid string, wr io.Writer, ctx context.Context) {

	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
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

	//fmt.Println(string(res.Json))
	err = json.Unmarshal(res.Json, &resp)

	if err != nil {
		panic(err)
	}

	c := resp.Content[0]
	for _, l := range c.Label {
		pickContent(c.Uid, &c.Content, l.Content, 6)
		fmt.Println(" =================== ")

		fmt.Println("Label: ")
		for _, c2 := range l.Content {

			fmt.Println(c2.Name)
		}
	}

	for _, l := range c.Artist {
		pickContent(c.Uid, &c.Content, l.Content, 10)
		fmt.Println(" =================== ")

		fmt.Println("Artist: ")
		for _, c2 := range l.Content {

			fmt.Println(c2.Name)
		}

	}
	for _, l := range c.WrittenBy {
		fmt.Println(" =================== ")

		fmt.Println("By: ")
		for _, c2 := range l.Content {

			fmt.Println(c2.Name)
		}
		pickContent(c.Uid, &c.Content, l.Content, 14)
	}

	fmt.Println(" =================== ")
	fmt.Println("EXTRA: ")

	for _, c2 := range resp.Extra {
		fmt.Println(c2.Name)
	}

	pickContent(c.Uid, &c.Content, resp.Extra, 16)

	rand.Shuffle(len(c.Content), func(i, j int) {
		c.Content[i], c.Content[j] = c.Content[j], c.Content[i]
	})

	updateRandom(c.Content, dg, ctx)

	fmt.Println(" =================== ")
	fmt.Println("Content: ")
	for _, c2 := range c.Content {

		fmt.Println(c2.Name)
	}

	fmt.Println("Spotify", c.Spotify)
	executeTemplate(wr, "content", c)
}

func printArtist(uid string, wr io.Writer, ctx context.Context) {

	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
	dg := dgo.NewDgraphClient(dc)

	q := `query Artist($terms: string) {
		artist(func: uid($terms)) @filter(type(Artist)) {
			uid
		   	name
		   	text
		  	pic
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

	executeTemplate(wr, "artist", resp.Artist[0])

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

func handler(w http.ResponseWriter, r *http.Request) {

	parts := strings.SplitN(r.URL.Path, "/", 4)
	if len(parts) >= 3 {
		switch parts[1] {
		case "artist":
			printArtist(parts[2], w, r.Context())
		case "content":
			printContent(parts[2], w, r.Context())
		case "search":
			printSearch(strings.Join(r.URL.Query()["terms"], " "), w, r.Context())
		default:
			printStart(w, r.Context())
		}
	} else {
		printStart(w, r.Context())
	}
	fmt.Println(r.URL)
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

func readCounter(w http.ResponseWriter, r *http.Request) {
	parts := strings.SplitN(r.URL.Path, "/", 5)
	if len(parts) < 4 {
		return
	}
	fmt.Printf("Read: %s", parts[2])
	updateCounter(parts[2], parts[3], r.Context())
}

func sse(w http.ResponseWriter, r *http.Request) {

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

	err = watcher.Add("./templates")
	if err != nil {
		panic(err)
	}
	err = watcher.Add("./css")
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
				w.Write([]byte("data: reload\n\n"))
				if f, ok := w.(http.Flusher); ok {
					f.Flush()
				}
				fmt.Println("Got event, sending...")
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

func updateCounter(uid, uuid string, ctx context.Context) {
	conn, err := grpc.Dial("127.0.0.1:9080", grpc.WithInsecure())
	if err != nil {
		log.Fatal("While trying to dial gRPC")
	}
	defer conn.Close()

	dc := api.NewDgraphClient(conn)
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

func main() {

	port = 9090

	updateCounter("0xbcc1", "jenson-uuid", context.Background())
	updateCounter("0xbcc4", "jenson-uuid", context.Background())

	http.Handle("/css/", http.StripPrefix("/css/", http.FileServer(http.Dir("./css"))))
	http.HandleFunc("/read/", readCounter)
	http.HandleFunc("/sse", sse)
	http.HandleFunc("/api/content/extra", apiExtraContent)
	http.HandleFunc("/", handler)                             // set router
	err := http.ListenAndServe(fmt.Sprintf(":%d", port), nil) // set listen port
	if err != nil {
		log.Fatal("ListenAndServe: ", err)
	}

}
