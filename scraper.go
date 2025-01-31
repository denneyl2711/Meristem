package main

import (
	"fmt"
	"io"
	"regexp"
	"strings"

	//Colly web scraping stuff
	"github.com/gocolly/colly"

	//for writing to console and updating written lines
	"github.com/gosuri/uilive"
)

var cw1 *CollyWrapper
var cw2 *CollyWrapper

func completeLink(url string) string {
	return "https://en.wikipedia.org" + url
}

func specialify(url string) string {
	return "/wiki/Special:WhatLinksHere/" + strings.TrimPrefix(url, "/wiki/")
}

func IsUrlAllowed(url string) bool {
	if !strings.HasPrefix(url, "/wiki") {
		return false
	}
	disallowedUrls := []*regexp.Regexp{
		regexp.MustCompile(`\/wiki\/Category`),
		regexp.MustCompile(`\/wiki\/Help`),
		regexp.MustCompile(`\/wiki\/Wikipedia`),
		regexp.MustCompile(`\/wiki\/Special`),
		regexp.MustCompile(`\/wiki\/Main_Page`),
		regexp.MustCompile(`\/wiki\/Template`),
		regexp.MustCompile(`\/wiki\/File`),
		regexp.MustCompile(`\/wiki\/Portal`),
		regexp.MustCompile(`\/wiki\/Talk`),
		regexp.MustCompile(`\/wiki\/Verifiability`),
		regexp.MustCompile(`\/wiki\/Notability`),
		regexp.MustCompile(`\/wiki\/Geographic_coordinate_system`),
		regexp.MustCompile(`\/wiki\/User`),
	}

	for _, regex := range disallowedUrls {
		if regex.MatchString(url) {
			return false
		}
	}
	return true
}

// this is called before the HTTP request is triggered
// an HTTP request is performed w/ Visit()

// in our case, the scraper only knows that we've clicked on the random tab
// (we haven't been redirected to the actual article yet)
func OnRequestFunc(r *colly.Request) {
	// fmt.Println("Visiting: ", r.URL)
	// fmt.Println()
}

// triggered when the scaper encounters an error
func OnErrorFunc(r *colly.Response, err error) {
	fmt.Println("Uh oh... ", err)
	fmt.Println("Response Status Code: ", r.StatusCode)
	fmt.Println("Failure with", r.Request.URL)
	fmt.Println()
}

var doneScanning bool = false

// need to have this variable to ensure that c2 only scans backwards (what links here)
// without this flag, c2 would scrape forwards for the first iteration
var enableC2Enqueue bool = false //TODO: do this cleaner

var writer *uilive.Writer
var writer2 io.Writer

// called once scraping is done
func OnScrapedFunc(r *colly.Response) {
	if doneScanning {
		return
	}

	startUrl := "Unknown"
	endUrl := "Unknown"

	if cw1.initNode != nil {
		startUrl = cw1.initNode.url
	}

	if cw2.initNode != nil {
		endUrl = cw2.initNode.url
	}

	fmt.Fprintln(writer)
	fmt.Fprintln(writer, "Start:", completeLink(startUrl))
	fmt.Fprintln(writer, "Target:", unspecialify(completeLink(endUrl)))
	fmt.Fprintln(writer)

	fmt.Fprintln(writer, "Collector 1 has seen", cw1.size(), "links")
	fmt.Fprintln(writer2, "Collector 2 has seen", cw2.size(), "links")

	if path := cw1.findConnection(cw2); path != nil {
		fmt.Fprintln(writer)
		for i := len(path) - 1; i >= 0; i-- {
			fmt.Fprintln(writer, unspecialify(completeLink(path[i].url)))
		}
		doneScanning = true
	} else {
		//continue recursing, if possible
		//TODO: make significantly less ugly
		if cw1.size() < cw2.size() {
			if !doneScanning && cw1.Dequeue() {
				cw1.collector.Visit(completeLink(cw1.currNode.url))
			}
			if !doneScanning && cw2.Dequeue() {
				enableC2Enqueue = true
				cw2.collector.Visit(completeLink(cw2.currNode.url))
			}
		} else {
			if !doneScanning && cw2.Dequeue() {
				enableC2Enqueue = true
				cw2.collector.Visit(completeLink(cw2.currNode.url))

				//nested to ensure that cw2 only continues scraping once cw1 has data
				if !doneScanning && cw1.Dequeue() {
					cw1.collector.Visit(completeLink(cw1.currNode.url))
				}
			}
		}
	}
}

func main() {
	writer = uilive.New()
	writer2 = writer.Newline()
	writer.Start()

	const recursionLimit int = 5

	var collector1 *colly.Collector = colly.NewCollector(
		//THE DOMAIN DOESN'T INCLUDE THE HTTP://
		colly.AllowedDomains("en.wikipedia.org"),
		colly.MaxDepth(recursionLimit),
		//this will print out debug info
		// colly.Debugger(&debug.LogDebugger{}),
	)

	collector2 := colly.NewCollector(
		colly.AllowedDomains("en.wikipedia.org"),
		colly.MaxDepth(recursionLimit),
	)

	collector1.OnRequest(OnRequestFunc)
	collector1.OnError(OnErrorFunc)
	collector1.OnScraped(OnScrapedFunc)

	collector2.OnRequest(OnRequestFunc)
	collector2.OnError(OnErrorFunc)
	collector2.OnScraped(OnScrapedFunc)

	cw1 = newCollyWrapper(collector1)
	cw2 = newCollyWrapper(collector2)

	//onResponse() is called right before onHTML()
	collector1.OnResponse(func(r *colly.Response) {
		url := r.Request.URL
		var node *LinkNode
		if cw1.initNode == nil {
			node = newNode(nil, url.Path, 0)
			cw1.setInitNode(node)
		}
	})

	collector2.OnResponse(func(r *colly.Response) {
		url := r.Request.URL.Path
		var node *LinkNode
		if cw2.initNode == nil {
			node = newNode(nil, specialify(url), 0)
			cw2.setInitNode(node)
			if cw2.Enqueue(node) {
				// fmt.Println("Enqueuing (OnResponse())", specialify(url))
			}
		}
	})

	// triggered when a CSS selector matches an element
	// this is called right after OnResponse() if the received content is HTML
	// collector1 enques the normal link
	collector1.OnHTML("a", func(elem *colly.HTMLElement) {
		//TODO: only enqueue items where the end of the href matches the title?
		//note: "matches" b/c "/wiki/Sicilian_language" != "Sicilian Language"
		//also, some links are valid but inside a hidden table or random things like that, need to account for those too
		url := elem.Attr("href")
		if !IsUrlAllowed(url) {
			return
		}
		newLink := newNode(cw1.currNode, url, cw1.currNode.distance+1)
		cw1.Enqueue(newLink)
	})

	// collector2 enques the special link
	collector2.OnHTML("a", func(elem *colly.HTMLElement) {
		if !enableC2Enqueue {
			return
		}
		url := elem.Attr("href")
		if !IsUrlAllowed(url) {
			return
		}
		if url == unspecialify(cw2.initNode.url) ||
			url == specialify(cw2.currNode.url) {
			return
		}
		newLink := newNode(cw2.currNode, specialify(url), cw2.currNode.distance+1)
		if cw2.Enqueue(newLink) {
			// fmt.Println("Enqueuing", url)
		}
	})

	collector1.Visit("https://en.wikipedia.org/wiki/Special:Random")
	collector2.Visit("https://en.wikipedia.org/wiki/Special:Random")

	// collector1.Visit("https://en.wikipedia.org/wiki/Sodesaki_Station")
	// collector2.Visit("https://en.wikipedia.org/wiki/47th_parallel_south")

	writer.Stop()
}
