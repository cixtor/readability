package readability

import (
	"bytes"

	"golang.org/x/net/html"
)

// appendChild adds a node to the end of the list of children of a specified
// parent node. If the given child is a reference to an existing node in the
// document, appendChild moves it from its current position to the new position
// (there is no requirement to remove the node from its parent node before
// appending it to some other node).
//
// See: https://developer.mozilla.org/en-US/docs/Web/API/Node/appendChild
func appendChild(node *html.Node, child *html.Node) {
	if child.Parent != nil {
		temp := cloneNode(child)
		node.AppendChild(temp)
		child.Parent.RemoveChild(child)
		return
	}

	node.AppendChild(child)
}

// childNodes returns list of a node's direct children.
func childNodes(node *html.Node) []*html.Node {
	var list []*html.Node

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		list = append(list, c)
	}

	return list
}

// cloneNode returns a duplicate of the node on which this method was called.
//
// See: https://developer.mozilla.org/en-US/docs/Web/API/Node/cloneNode
func cloneNode(node *html.Node) *html.Node {
	clone := &html.Node{
		Type:     node.Type,
		DataAtom: node.DataAtom,
		Data:     node.Data,
		Attr:     make([]html.Attribute, len(node.Attr)),
	}

	copy(clone.Attr, node.Attr)

	for c := node.FirstChild; c != nil; c = c.NextSibling {
		clone.AppendChild(cloneNode(c))
	}

	return clone
}

// createElement creates the HTML element specified by tagName.
//
// See: https://developer.mozilla.org/en-US/docs/Web/API/Document/createElement
func createElement(tagName string) *html.Node {
	return &html.Node{Type: html.ElementNode, Data: tagName}
}

// getElementsByTagName returns a collection of HTML elements with the given
// tag name. If tag name is an asterisk, a list of all the available HTML nodes
// will be returned instead.
//
// See: https://developer.mozilla.org/en-US/docs/Web/API/Document/getElementsByTagName
func getElementsByTagName(node *html.Node, tag string) []*html.Node {
	var lst []*html.Node
	var fun func(*html.Node)

	fun = func(n *html.Node) {
		if n.Type == html.ElementNode && (tag == "*" || n.Data == tag) {
			lst = append(lst, n)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			fun(c)
		}
	}

	fun(node)

	return lst
}

// indexOf returns the first index at which a given element can be found in the
// array, or -1 if it is not present.
func indexOf(array []string, key string) int {
	for idx, val := range array {
		if val == key {
			return idx
		}
	}

	return -1
}

// replaceNode replaces a child node within the given (parent) node.
//
// See: https://developer.mozilla.org/en-US/docs/Web/API/Node/replaceChild
func replaceNode(oldNode *html.Node, newNode *html.Node) {
	if oldNode.Parent == nil {
		return
	}

	newNode.Parent = nil
	newNode.PrevSibling = nil
	newNode.NextSibling = nil
	oldNode.Parent.InsertBefore(newNode, oldNode)
	oldNode.Parent.RemoveChild(oldNode)
}
// textContent returns text content of a node and its descendants.
//
// See: https://developer.mozilla.org/en-US/docs/Web/API/Node/textContent
func textContent(node *html.Node) string {
	var buf bytes.Buffer
	var fun func(*html.Node)

	fun = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			fun(c)
		}
	}

	fun(node)

	return buf.String()
}
