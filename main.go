package main

import (
	"flag"
	"gopkg.in/tylerb/graceful.v1"
	"log"
	"net/http"
	"regexp"
	"time"
	"strconv"
	"github.com/shesuyo/go-fastcgi-client"
)

var (
	statusLineRegexp = regexp.MustCompile(`(?m)^(.*):\s+(.*)$`)
	fpmStatusURL     = ""
	fpmPort          = 9000
	listenAddr       = ""
)

func main() {
	port  := flag.String("port", "9000", "PHP-FPM server port")
	url  := flag.String("status-url", "/status", "PHP-FPM status URL")
	addr := flag.String("addr", "0.0.0.0:9095", "IP/port for the HTTP server")
	flag.Parse()

	if *port != "" {
		p, err := strconv.Atoi(*port)
		if err != nil {
			log.Fatal("Bad value for port")
		}
    fpmPort = *p
	}

	if *url == "" {
		log.Fatal("The status-url flag is required.")
	} else {
		fpmStatusURL = *url
	}

	if *addr == "" {
		listenAddr = "0.0.0.0:9095"
	} else {
		listenAddr = *addr
	}

	scrapeFailures := 0

	server := &graceful.Server{
		Timeout: 10 * time.Second,
		Server: &http.Server{
			Addr:        listenAddr,
			ReadTimeout: time.Duration(5) * time.Second,
			Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

				env := make(map[string]string)
				env["REQUEST_METHOD"] = "GET"
				env["SCRIPT_FILENAME"] = fpmStatusURL
				env["SCRIPT_NAME"] = fpmStatusURL
				env["SERVER_SOFTWARE"] = "go / fcgiclient "
				env["SERVER_PROTOCOL"] = "HTTP/1.1"
				env["QUERY_STRING"] = ""

				fcgi, err := fcgiclient.New("127.0.0.1", fpmPort)
				if err != nil {
					log.Println(err)
					scrapeFailures = scrapeFailures+1
					x := strconv.Itoa(scrapeFailures)
					NewMetricsFromMatches([][]string{{"scrape failure:","scrape failure",x}}).WriteTo(w)
					return
				}

				resp, err := fcgi.Request(env, "")
				if err != nil {
					log.Println(err)
					scrapeFailures = scrapeFailures+1
					x := strconv.Itoa(scrapeFailures)
					NewMetricsFromMatches([][]string{{"scrape failure:","scrape failure",x}}).WriteTo(w)
					return
				}

				x := strconv.Itoa(scrapeFailures)

				matches := statusLineRegexp.FindAllStringSubmatch(string(resp), -1)
				matches = append(matches,[]string{"scrape failure:","scrape failure",x})

				NewMetricsFromMatches(matches).WriteTo(w)
			}),
		},
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
