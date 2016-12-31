package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Quote struct {
	ID   int
	Text string
	Name string
}

type QuotationPage struct {
	Quotations []QuotationBlock
	Topic      string
	About      bool
}

type QuotationBlock struct {
	Html  template.HTML
	Name  template.HTML
	Topic string
}

var quotes = struct {
	sync.RWMutex
	q []Quote
}{q: make([]Quote, 0)}

var templates = template.Must(template.ParseFiles("quotes.html"))

func renderTemplate(w http.ResponseWriter, tmpl string, q *QuotationPage) {
	err := templates.ExecuteTemplate(w, tmpl+".html", q)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

var validPath = regexp.MustCompile("^/(about|subject|author|random)/([.a-zA-Z0-9 -]+)$")

func makeHandler(fn func(http.ResponseWriter, *http.Request, string, string)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		m := validPath.FindStringSubmatch(r.URL.Path)
		if m == nil {
			http.Redirect(w, r, "/random/1", http.StatusFound)
			return
		}
		defer timeTrack(time.Now(), r.URL.Path)
		fn(w, r, m[1], m[2])
	}
}

func quotationHandler(w http.ResponseWriter, r *http.Request, route string, topic string) {

	isJson := false
	if strings.Contains(topic, ".json") {
		isJson = true
		topic = strings.Replace(topic, ".json", "", -1)
	}
	topic = strings.ToLower(topic)

	// Get quotes
	var quotes []Quote
	if route == "random" {
		num, err := strconv.ParseInt(topic, 10, 64)
		if err != nil {
			num = 1
		}
		quotes = getRandomQuotes(int(num))
	} else {
		quotes = getQuotesFromIndex(topic)
	}

	// Make HTML for quotes
	qs := QuotationPage{Quotations: make([]QuotationBlock, len(quotes))}
	for i := range qs.Quotations {
		htmlString := ""
		for _, field := range strings.Fields(quotes[i].Text) {
			if isStopWord(field) {
				htmlString += field + " "
			} else {
				htmlString += fmt.Sprintf("<a href='/subject/%s'>%s</a> ", cleanString(strings.ToLower(field)), field)
			}
		}
		htmlName := ""
		if len(quotes[i].Name) > 0 {
			htmlName = fmt.Sprintf("<a href='/author/%s'>%s</a> ", strings.ToLower(quotes[i].Name), quotes[i].Name)
		}
		qs.Quotations[i] = QuotationBlock{
			Html: template.HTML(htmlString),
			Name: template.HTML(htmlName),
		}
	}

	if isJson {
		jData, _ := json.MarshalIndent(quotes, "", "  ")
		w.Header().Set("Content-Type", "application/json")
		w.Write(jData)
	} else {
		if route != "random" {
			qs.Topic = strings.Title(topic)
		}
		renderTemplate(w, "quotes", &qs)
	}
}

func indexHandler(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/random/1", http.StatusFound)
	return
}

func searchHandler(w http.ResponseWriter, r *http.Request) {
	term := strings.TrimSpace(strings.ToLower(r.FormValue("term")))
	http.Redirect(w, r, "/subject/"+term, http.StatusFound)
}

func aboutHandler(w http.ResponseWriter, r *http.Request) {
	qs := QuotationPage{Quotations: make([]QuotationBlock, 0), About: true}
	renderTemplate(w, "quotes", &qs)
}

var Port string
var Dump bool

func main() {
	flag.StringVar(&Port, "port", "8014", "port to run this server on (default: 8072)")
	flag.BoolVar(&Dump, "dump", false, "dump database")
	flag.Parse()

	if Dump {
		dumpDatabase()
		os.Exit(1)
	}

	if _, err := os.Stat("quotations.db"); os.IsNotExist(err) {
		buildDatabase()
		os.Exit(1)
	}

	http.HandleFunc("/subject/", makeHandler(quotationHandler))
	http.HandleFunc("/author/", makeHandler(quotationHandler))
	http.HandleFunc("/random/", makeHandler(quotationHandler))
	http.HandleFunc("/search/", searchHandler)
	http.HandleFunc("/", indexHandler)
	http.HandleFunc("/about/", aboutHandler)
	log.Printf("Running at 0.0.0.0:" + Port)
	http.ListenAndServe(":"+Port, nil)
}
