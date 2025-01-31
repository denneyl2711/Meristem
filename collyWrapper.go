package main

import (
	"fmt"
	"strings"

	mapset "github.com/deckarep/golang-set/v2"

	// Colly web scraping stuff
	"github.com/gocolly/colly"
)

var Reset = "\033[0m"
var Red = "\033[31m"
var Green = "\033[32m"
var Yellow = "\033[33m"
var Blue = "\033[34m"
var Magenta = "\033[35m"
var Cyan = "\033[36m"
var Gray = "\033[37m"
var White = "\033[97m"

type CollyWrapper struct {
	initNode *LinkNode
	currNode *LinkNode

	nodes     []*LinkNode        //history of all nodes which have been visited --> THESE URL'S MAY HAVE THE SPECIAL TAG
	linkQueue []*LinkNode        //list of nodes which need to be explored --> THESE URL'S MAY HAVE THE SPECIAL TAG
	urlSet    mapset.Set[string] //unique list of urls that have been visited --> THESE URL'S DO NOT HAVE THE SPECIAL TAG

	setSize int //have to do this manually for some reason

	collector *colly.Collector
}

func newCollyWrapper(collector *colly.Collector) *CollyWrapper {
	newCollector := new(CollyWrapper)
	newCollector.collector = collector
	newCollector.urlSet = mapset.NewSet[string]()
	newCollector.setSize = 0

	return newCollector
}

func unspecialify(url string) string {
	return strings.ReplaceAll(url, "/Special:WhatLinksHere", "")
}

func (cw *CollyWrapper) Enqueue(node *LinkNode) bool {
	unspecialedLink := unspecialify(node.url)
	if cw.urlSet.Add(unspecialedLink) {
		cw.nodes = append(cw.nodes, node)
		cw.linkQueue = append(cw.linkQueue, node)
		cw.setSize++
		return true
	}
	return false
}

func (cw *CollyWrapper) Dequeue() bool {
	if len(cw.linkQueue) == 0 {
		return false
	}
	cw.currNode = cw.linkQueue[0]
	cw.linkQueue = cw.linkQueue[1:]
	return true
}

func (cw *CollyWrapper) setInitNode(node *LinkNode) {
	cw.initNode = node
	cw.currNode = node
}

func (cw *CollyWrapper) findConnection(other *CollyWrapper) []*LinkNode {
	if cw.currNode == nil || other.currNode == nil {
		return nil
	}

	intersection := cw.urlSet.Intersect(other.urlSet)
	if intersection.Cardinality() == 0 {
		return nil
	}

	//if the articles are right next to each other, then just return the two articles
	if cw.urlSet.Contains(other.initNode.url) &&
		other.urlSet.Contains(cw.initNode.url) {
		returned := make([]*LinkNode, 0)
		returned = append(returned, cw.initNode)
		returned = append(returned, other.initNode)
		return returned
	}

	//find each wrapper's path to the thing in the set
	var middle_url string = intersection.ToSlice()[0]

	//calculate cw path
	fmt.Println()
	path := make([]*LinkNode, 0)
	for _, link := range cw.nodes {
		if link.url == middle_url {
			tempNode := link
			for tempNode.prevNode != nil {
				tempNode = tempNode.prevNode
				path = append(path, tempNode)
			}
		}
	}

	//calculate other path
	for _, linkNode := range other.nodes {
		//other has the /Special:WhatLinksHere/ links, so we need to convert those
		unspecialifiedLink := unspecialify(linkNode.url)
		if unspecialifiedLink == middle_url {
			tempNode := linkNode
			path = append([]*LinkNode{tempNode}, path...)
			for tempNode.prevNode != nil {
				tempNode = tempNode.prevNode //"prevNode" is really the next node
				//add to the front of the list to maintain proper order
				path = append([]*LinkNode{tempNode}, path...)
			}
		}
	}

	return path
}

func (cw *CollyWrapper) size() int {
	return cw.setSize
}
