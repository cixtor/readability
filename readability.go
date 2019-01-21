package readability

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/net/html"
)

// All of the regular expressions in use within readability.
// Defined up here so we don't instantiate them repeatedly in loops.
var rxWhitespace = regexp.MustCompile(`^\s*$`)

// The commented out elements qualify as phrasing content but tend to be
// removed by readability when put into paragraphs, so we ignore them here.
var phrasingElems = []string{
	// "canvas", "iframe", "svg", "video",
	"abbr", "audio", "b", "bdo", "br", "button", "cite", "code", "data",
	"datalist", "dfn", "em", "embed", "i", "img", "input", "kbd", "label",
	"mark", "math", "meter", "noscript", "object", "output", "progress", "q",
	"ruby", "samp", "script", "select", "small", "span", "strong", "sub",
	"sup", "textarea", "time", "var", "wbr",
}

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

// replaceNodeTags iterates over a list, and calls setNodeTag for each node.
func (r *Readability) replaceNodeTags(list []*html.Node, newTagName string) {
	for i := len(list) - 1; i >= 0; i-- {
		node := list[i]
		r.setNodeTag(node, newTagName)
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

// prepDocument prepares the HTML document for readability to scrape it. This
// includes things like stripping JavaScript, CSS, and handling terrible markup
// among other things.
func (r *Readability) prepDocument() {
	doc := r.doc

	r.removeNodes(getElementsByTagName(doc, "style"), nil)

	if n := getElementsByTagName(doc, "body"); len(n) > 0 && n[0] != nil {
		r.replaceBrs(n[0])
	}

	r.replaceNodeTags(getElementsByTagName(doc, "font"), "SPAN")
}

// nextElement finds the next element, starting from the given node, and
// ignoring whitespace in between. If the given node is an element, the same
// node is returned.
func (r *Readability) nextElement(node *html.Node) *html.Node {
	next := node

	for next != nil &&
		next.Type != html.ElementNode &&
		rxWhitespace.MatchString(textContent(next)) {
		next = next.NextSibling
	}

	return next
}

// replaceBrs replaces two or more successive <br> elements with a single <p>.
// Whitespace between <br> elements are ignored. For example:
//
//   <div>foo<br>bar<br> <br><br>abc</div>
//
// will become:
//
//   <div>foo<br>bar<p>abc</p></div>
func (r *Readability) replaceBrs(elem *html.Node) {
	r.forEachNode(r.getAllNodesWithTag(elem, "br"), func(br *html.Node, _ int) {
		next := br.NextSibling

		// Whether two or more <br> elements have been found and replaced with
		// a <p> block.
		replaced := false

		// If we find a <br> chain, remove the <br> nodes until we hit another
		// element or non-whitespace. This leaves behind the first <br> in the
		// chain (which will be replaced with a <p> later).
		for {
			next = r.nextElement(next)

			if next == nil || tagName(next) == "BR" {
				break
			}

			replaced = true
			brSibling := next.NextSibling
			next.Parent.RemoveChild(next)
			next = brSibling
		}

		// If we removed a <br> chain, replace the remaining <br> with a <p>.
		// Add all sibling nodes as children of the <p> until we hit another
		// <br> chain.
		if replaced {
			p := createElement("p")
			replaceNode(br, p)

			next = p.NextSibling
			for next != nil {
				// If we have hit another <br><br>, we are done adding children
				// to this <p>.
				if tagName(next) == "br" {
					nextElem := r.nextElement(next.NextSibling)
					if nextElem != nil && tagName(nextElem) == "br" {
						break
					}
				}

				if !r.isPhrasingContent(next) {
					break
				}

				// Otherwise, make this node a child of the new <p>.
				sibling := next.NextSibling
				appendChild(p, next)
				next = sibling
			}

			for p.LastChild != nil && r.isWhitespace(p.LastChild) {
				p.RemoveChild(p.LastChild)
			}

			if tagName(p.Parent) == "P" {
				r.setNodeTag(p.Parent, "div")
			}
		}
	})
}

func (r *Readability) setNodeTag(node *html.Node, newTagName string) {
	if node.Type == html.ElementNode {
		node.Data = newTagName
	}

	// NOTES(cixtor): the original function in Readability.js is a bit longer
	// because it contains a fallback mechanism to set the node tag name just
	// in case JSDOMParser is not available, there is no need to implement this
	// here.
}

// removeScripts removes script tags from the document.
func (r *Readability) removeScripts(doc *html.Node) {
	r.removeNodes(getElementsByTagName(doc, "script"), nil)
	r.removeNodes(getElementsByTagName(doc, "noscript"), nil)
}

// isPhrasingContent determines if a node qualifies as phrasing content.
//
// See: https://developer.mozilla.org/en-US/docs/Web/Guide/HTML/Content_categories#Phrasing_content
func (r *Readability) isPhrasingContent(node *html.Node) bool {
	if node.Type == html.TextNode {
		return true
	}

	tag := tagName(node)

	if indexOf(phrasingElems, tag) != -1 {
		return true
	}

	return ((tag == "a" || tag == "del" || tag == "ins") &&
		r.everyNode(childNodes(node), r.isPhrasingContent))
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

	r.prepDocument()

	return Article{}, nil
}
