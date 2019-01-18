package readability

import (
	"fmt"
	"io"
	"net/url"

	"golang.org/x/net/html"
)

type Readability struct {
	doc *html.Node
	uri *url.URL

	// MaxElemsToParse is the optional maximum number of HTML nodes to parse
	// from the document. If the number of elements in the document is higher
	// than this number, the operation immediately errors.
	MaxElemsToParse int
}

// Article represents the metadata and content of the article.
type Article struct {
	// Title is the heading that preceeds the article’s content, and the basis
	// for the article’s page name and URL. It indicates what the article is
	// about, and distinguishes it from other articles. The title may simply
	// be the name of the subject of the article, or it may be a description
	// of the topic.
	Title string

	// Byline is a printed line of text accompanying a news story, article, or
	// the like, giving the author’s name
	Byline string

	// Dir is the direction of the text in the article.
	//
	// Either Left-to-Right (LTR) or Right-to-Left (RTL).
	Dir string

	// Content is the relevant text in the article with HTML tags.
	Content string

	// TextContent is the relevant text in the article without HTML tags.
	TextContent string

	// Excerpt is the summary for the relevant text in the article.
	Excerpt string

	// SiteName is the name of the original publisher website.
	SiteName string

	// Favicon (short for favorite icon) is a file containing one or more small
	// icons, associated with a particular website or web page. A web designer
	// can create such an icon and upload it to a website (or web page) by
	// several means, and graphical web browsers will then make use of it.
	Favicon string

	// Image is an image URL which represents the article’s content.
	Image string

	// Length is the amount of characters in the article.
	Length int
}

func New(reader io.Reader, rawurl string) (Readability, error) {
	var err error
	var r Readability

	if r.uri, err = url.ParseRequestURI(rawurl); err != nil {
		return Readability{}, fmt.Errorf("url.ParseRequestURI %s", err)
	}

	if r.doc, err = html.Parse(reader); err != nil {
		return Readability{}, fmt.Errorf("html.Parse %s", err)
	}

	return r, nil
}

// removeNodes iterates over a collection of HTML elements, calls the optional
// filter function on each node, and removes the node if function returns True.
// If function is not passed, removes all the nodes in the list.
func (r *Readability) removeNodes(list []*html.Node, filter func(*html.Node) bool) {
	var node *html.Node
	var parentNode *html.Node

	for i := len(list) - 1; i >= 0; i-- {
		node = list[i]
		parentNode = node.Parent

		if parentNode != nil && (filter == nil || filter(node)) {
			parentNode.RemoveChild(node)
		}
	}
}

// forEachNode iterates over a list of HTML nodes, which doesn’t natively fully
// implement the Array interface. For convenience, the current object context
// is applied to the provided iterate function.
func (r *Readability) forEachNode(list []*html.Node, fn func(*html.Node, int)) {
	for idx, node := range list {
		fn(node, idx)
	}
}

// everyNode iterates over a collection of nodes, returns true if all of the
// provided iterator function calls return true, otherwise returns false. For
// convenience, the current object context is applied to the provided iterator
// function.
func (r *Readability) everyNode(list []*html.Node, fn func(*html.Node) bool) bool {
	for _, node := range list {
		if !fn(node) {
			return false
		}
	}

	return true
}

func (r *Readability) getAllNodesWithTag(node *html.Node, tagNames ...string) []*html.Node {
	var list []*html.Node

	for _, tag := range tagNames {
		list = append(list, getElementsByTagName(node, tag)...)
	}

	return list
}
// removeScripts removes script tags from the document.
func (r *Readability) removeScripts(doc *html.Node) {
	r.removeNodes(getElementsByTagName(doc, "script"), nil)
	r.removeNodes(getElementsByTagName(doc, "noscript"), nil)
}


func (r *Readability) isWhitespace(node *html.Node) bool {
	if node.Type == html.TextNode && strings.TrimSpace(textContent(node)) == "" {
		return true
	}

	return node.Type == html.ElementNode && tagName(node) == "br"
}

// Parse runs readability.
func (r *Readability) Parse() (Article, error) {
	if r.MaxElemsToParse > 0 {
		numTags := len(getElementsByTagName(r.doc, "*"))
		if numTags > r.MaxElemsToParse {
			return Article{}, fmt.Errorf("aborting parsing document; %d elements found", numTags)
		}
	}

	r.removeScripts(r.doc)

	return Article{}, nil
}
