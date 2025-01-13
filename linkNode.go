package main

type LinkNode struct {
	prevNode *LinkNode
	url      string
	distance int
}

func newNode(prevNode *LinkNode, url string, distance int) *LinkNode {
	newNode := new(LinkNode)

	newNode.prevNode = prevNode
	newNode.url = url
	newNode.distance = distance

	return newNode
}
