package main

import (
	"flag"
	"gopkg.in/tylerb/graceful.v1"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"time"
	"strconv"
	"github.com/tomasen/fcgi_client"
)

var (
	statusLineRegexp = regexp.MustCompile(`(?m)^(.*):\s+(.*)$`)
	fpmStatusURL     = ""
	fpmServer        = ""
	listenAddr       = ""
)

func main() {
	srv  := flag.String("fpm-server", "", "PHP-FPM server")
	url  := flag.String("status-url", "", "PHP-FPM status URL")
	addr := flag.String("addr", "0.0.0.0:9095", "IP/port for the HTTP server")
	flag.Parse()

	if *srv == "" {
		log.Fatal("The server flag is required.")
	} else {
		fpmServer = *srv
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
				env["SCRIPT_NAME"] = fpmStatusURL
				env["SCRIPT_FILENAME"] = fpmStatusURL

				fcgi, err := fcgiclient.Dial("tcp", fpmServer)
				if err != nil {
					log.Println(err)
					scrapeFailures = scrapeFailures+1
					x := strconv.Itoa(scrapeFailures)
					NewMetricsFromMatches([][]string{{"scrape failure:","scrape failure",x}}).WriteTo(w)
					return
				}

				resp, err := fcgi.Get(env)
				fcgi.Close()
				if err != nil {
					log.Println(err)
					scrapeFailures = scrapeFailures+1
					x := strconv.Itoa(scrapeFailures)
					NewMetricsFromMatches([][]string{{"scrape failure:","scrape failure",x}}).WriteTo(w)
					return
				}

				if (resp.StatusCode != http.StatusOK){
					log.Println("php-fpm status code is not OK.")
					scrapeFailures = scrapeFailures+1
					x := strconv.Itoa(scrapeFailures)
					NewMetricsFromMatches([][]string{{"scrape failure:","scrape failure",x}}).WriteTo(w)
					return
				}

				body, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					log.Println(err)
					scrapeFailures = scrapeFailures+1
					x := strconv.Itoa(scrapeFailures)
					NewMetricsFromMatches([][]string{{"scrape failure:","scrape failure",x}}).WriteTo(w)
					return
				}

				resp.Body.Close()

				x := strconv.Itoa(scrapeFailures)

				matches := statusLineRegexp.FindAllStringSubmatch(string(body), -1)
				matches = append(matches,[]string{"scrape failure:","scrape failure",x})

				NewMetricsFromMatches(matches).WriteTo(w)
			}),
		},
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatal(err)
	}
}
