package readability

import (
	"fmt"
	"io"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// All of the regular expressions in use within readability.
// Defined up here so we don't instantiate them repeatedly in loops.
var rxPositive = regexp.MustCompile(`(?i)article|body|content|entry|hentry|h-entry|main|page|pagination|post|text|blog|story`)
var rxNegative = regexp.MustCompile(`(?i)hidden|^hid$| hid$| hid |^hid |banner|combx|comment|com-|contact|foot|footer|footnote|gdpr|masthead|media|meta|outbrain|promo|related|scroll|share|shoutbox|sidebar|skyscraper|sponsor|shopping|tags|tool|widget`)
var rxByline = regexp.MustCompile(`(?i)byline|author|dateline|writtenby|p-author`)
var rxNormalize = regexp.MustCompile(`(?i)\s{2,}`)
var rxWhitespace = regexp.MustCompile(`(?i)^\s*$`)
var rxHasContent = regexp.MustCompile(`(?i)\S$`)
var rxPropertyPattern = regexp.MustCompile(`(?i)\s*(dc|dcterm|og|twitter)\s*:\s*(author|creator|description|title|site_name|image\S*)\s*`)
var rxNamePattern = regexp.MustCompile(`(?i)^\s*(?:(dc|dcterm|og|twitter|weibo:(article|webpage))\s*[\.:]\s*)?(author|creator|description|title|site_name|image)\s*$`)
var rxTitleSeparator = regexp.MustCompile(`(?i) [\|\-\\/>»] `)
var rxTitleHierarchySep = regexp.MustCompile(`(?i) [\\/>»] `)
var rxTitleRemoveFinalPart = regexp.MustCompile(`(?i)(.*)[\|\-\\/>»] .*`)
var rxTitleRemove1stPart = regexp.MustCompile(`(?i)[^\|\-\\/>»]*[\|\-\\/>»](.*)`)
var rxTitleAnySeparator = regexp.MustCompile(`(?i)[\|\-\\/>»]+`)
var rxDisplayNone = regexp.MustCompile(`(?i)display\s*:\s*none`)
var rxFaviconSize = regexp.MustCompile(`(?i)(\d+)x(\d+)`)

// divToPElems is a list of HTML tag names representing content dividers.
var divToPElems = []string{
	"a", "blockquote", "div", "dl", "img",
	"ol", "p", "pre", "select", "table", "ul",
}

// presentationalAttributes is a list of HTML attributes used to style Nodes.
var presentationalAttributes = []string{
	"align",
	"background",
	"bgcolor",
	"border",
	"cellpadding",
	"cellspacing",
	"frame",
	"hspace",
	"rules",
	"style",
	"valign",
	"vspace",
}

// deprecatedSizeAttributeElems is a list of HTML tags that allow programmers
// to set Width and Height attributes to define their own size but that have
// already been deprecated in recent HTML specifications.
var deprecatedSizeAttributeElems = []string{
	"table",
	"th",
	"td",
	"hr",
	"pre",
}

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

// flags is flags that used by parser.
type flags struct {
	useWeightClasses   bool
}

type Readability struct {
	doc           *html.Node
	documentURI   *url.URL
	articleByline string
	flags         flags

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

	if r.documentURI, err = url.ParseRequestURI(rawurl); err != nil {
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

// someNode iterates over a NodeList, return true if any of the
// provided iterate function calls returns true, false otherwise.
func (r *Readability) someNode(nodeList []*html.Node, fn func(*html.Node) bool) bool {
	for i := 0; i < len(nodeList); i++ {
		if fn(nodeList[i]) {
			return true
		}
	}

	return false
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

// concatNodeLists concats all nodelists passed as arguments.
func (r *Readability) concatNodeLists(nodeLists ...[]*html.Node) []*html.Node {
	var result []*html.Node

	for i := 0; i < len(nodeLists); i++ {
		result = append(result, nodeLists[i]...)
	}

	return result
}

func (r *Readability) getAllNodesWithTag(node *html.Node, tagNames ...string) []*html.Node {
	var list []*html.Node

	for _, tag := range tagNames {
		list = append(list, getElementsByTagName(node, tag)...)
	}

	return list
}

// getArticleTitle attempts to get the article title.
func (r *Readability) getArticleTitle() string {
	doc := r.doc
	curTitle := ""
	origTitle := ""
	titleHadHierarchicalSeparators := false

	// If they had an element with tag "title" in their HTML
	if nodes := getElementsByTagName(doc, "title"); len(nodes) > 0 {
		origTitle = r.getInnerText(nodes[0], true)
		curTitle = origTitle
	}

	// If there's a separator in the title, first remove the final part
	if rxTitleSeparator.MatchString(curTitle) {
		titleHadHierarchicalSeparators = rxTitleHierarchySep.MatchString(curTitle)
		curTitle = rxTitleRemoveFinalPart.ReplaceAllString(origTitle, "$1")

		// If the resulting title is too short (3 words or fewer), remove
		// the first part instead:
		if wordCount(curTitle) < 3 {
			curTitle = rxTitleRemove1stPart.ReplaceAllString(origTitle, "$1")
		}
	} else if strings.Index(curTitle, ": ") != -1 {
		// Check if we have an heading containing this exact string, so
		// we could assume it's the full title.
		headings := r.concatNodeLists(
			getElementsByTagName(doc, "h1"),
			getElementsByTagName(doc, "h2"),
		)

		trimmedTitle := strings.TrimSpace(curTitle)
		match := r.someNode(headings, func(heading *html.Node) bool {
			return strings.TrimSpace(textContent(heading)) == trimmedTitle
		})

		// If we don't, let's extract the title out of the original
		// title string.
		if !match {
			curTitle = origTitle[strings.LastIndex(origTitle, ":")+1:]

			// If the title is now too short, try the first colon instead:
			if wordCount(curTitle) < 3 {
				curTitle = origTitle[strings.Index(origTitle, ":")+1:]
				// But if we have too many words before the colon there's
				// something weird with the titles and the H tags so let's
				// just use the original title instead
			} else if wordCount(origTitle[:strings.Index(origTitle, ":")]) > 5 {
				curTitle = origTitle
			}
		}
	} else if len(curTitle) > 150 || len(curTitle) < 15 {
		if hOnes := getElementsByTagName(doc, "h1"); len(hOnes) == 1 {
			curTitle = r.getInnerText(hOnes[0], true)
		}
	}

	curTitle = strings.TrimSpace(curTitle)
	curTitle = rxNormalize.ReplaceAllString(curTitle, " ")
	// If we now have 4 words or fewer as our title, and either no
	// 'hierarchical' separators (\, /, > or ») were found in the original
	// title or we decreased the number of words by more than 1 word, use
	// the original title.
	curTitleWordCount := wordCount(curTitle)
	tmpOrigTitle := rxTitleAnySeparator.ReplaceAllString(origTitle, "")

	if curTitleWordCount <= 4 &&
		(!titleHadHierarchicalSeparators ||
			curTitleWordCount != wordCount(tmpOrigTitle)-1) {
		curTitle = origTitle
	}

	return curTitle
}

// getArticleFavicon attempts to get high quality favicon
// that used in article. It will only pick favicon in PNG
// format, so small favicon that uses ico file won't be picked.
// Using algorithm by philippe_b.
func (r *Readability) getArticleFavicon() string {
	favicon := ""
	faviconSize := -1
	linkElements := getElementsByTagName(r.doc, "link")

	r.forEachNode(linkElements, func(link *html.Node, _ int) {
		linkRel := strings.TrimSpace(getAttribute(link, "rel"))
		linkType := strings.TrimSpace(getAttribute(link, "type"))
		linkHref := strings.TrimSpace(getAttribute(link, "href"))
		linkSizes := strings.TrimSpace(getAttribute(link, "sizes"))

		if linkHref == "" || !strings.Contains(linkRel, "icon") {
			return
		}

		if linkType != "image/png" && !strings.Contains(linkHref, ".png") {
			return
		}

		size := 0
		for _, sizesLocation := range []string{linkSizes, linkHref} {
			sizeParts := rxFaviconSize.FindStringSubmatch(sizesLocation)
			if len(sizeParts) != 3 || sizeParts[1] != sizeParts[2] {
				continue
			}

			size, _ = strconv.Atoi(sizeParts[1])
			break
		}

		if size > faviconSize {
			faviconSize = size
			favicon = linkHref
		}
	})

	return toAbsoluteURI(favicon, r.documentURI)
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

// getArticleMetadata attempts to get excerpt and byline metadata for the article.
func (r *Readability) getArticleMetadata() Article {
	values := make(map[string]string)
	metaElements := getElementsByTagName(r.doc, "meta")

	// Find description tags.
	r.forEachNode(metaElements, func(element *html.Node, _ int) {
		elementName := getAttribute(element, "name")
		elementProperty := getAttribute(element, "property")
		content := getAttribute(element, "content")
		if content == "" {
			return
		}
		matches := []string{}
		name := ""

		if elementProperty != "" {
			matches = rxPropertyPattern.FindAllString(elementProperty, -1)
			for i := len(matches) - 1; i >= 0; i-- {
				// Convert to lowercase, and remove any whitespace
				// so we can match belops.
				name = strings.ToLower(matches[i])
				name = strings.Join(strings.Fields(name), "")
				// multiple authors
				values[name] = strings.TrimSpace(content)
			}
		}

		if len(matches) == 0 && elementName != "" && rxNamePattern.MatchString(elementName) {
			// Convert to lowercase, remove any whitespace, and convert
			// dots to colons so we can match belops.
			name = strings.ToLower(elementName)
			name = strings.Join(strings.Fields(name), "")
			name = strings.Replace(name, ".", ":", -1)
			values[name] = strings.TrimSpace(content)
		}
	})

	// get title
	metadataTitle := ""
	for _, name := range []string{
		"dc:title",
		"dcterm:title",
		"og:title",
		"weibo:article:title",
		"weibo:webpage:title",
		"title",
		"twitter:title",
	} {
		if value, ok := values[name]; ok {
			metadataTitle = value
			break
		}
	}

	if metadataTitle == "" {
		metadataTitle = r.getArticleTitle()
	}

	// get author
	metadataByline := ""
	for _, name := range []string{
		"dc:creator",
		"dcterm:creator",
		"author",
	} {
		if value, ok := values[name]; ok {
			metadataByline = value
			break
		}
	}

	// get description
	metadataExcerpt := ""
	for _, name := range []string{
		"dc:description",
		"dcterm:description",
		"og:description",
		"weibo:article:description",
		"weibo:webpage:description",
		"description",
		"twitter:description",
	} {
		if value, ok := values[name]; ok {
			metadataExcerpt = value
			break
		}
	}

	// get site name
	metadataSiteName := values["og:site_name"]

	// get image thumbnail
	metadataImage := ""
	for _, name := range []string{
		"og:image",
		"image",
		"twitter:image",
	} {
		if value, ok := values[name]; ok {
			metadataImage = toAbsoluteURI(value, r.documentURI)
			break
		}
	}

	// get favicon
	metadataFavicon := r.getArticleFavicon()

	return Article{
		Title:    metadataTitle,
		Byline:   metadataByline,
		Excerpt:  metadataExcerpt,
		SiteName: metadataSiteName,
		Image:    metadataImage,
		Favicon:  metadataFavicon,
	}
}

// initializeNode initializes a node with the readability score. Also checks
// the className/id for special names to add to its score.
func (r *Readability) initializeNode(node *html.Node) {
	contentScore := float64(r.getClassWeight(node))

	switch tagName(node) {
	case "div":
		contentScore += 5
	case "pre", "td", "blockquote":
		contentScore += 3
	case "address", "ol", "ul", "dl", "dd", "dt", "li", "form":
		contentScore -= 3
	case "h1", "h2", "h3", "h4", "h5", "h6", "th":
		contentScore -= 5
	}

	r.setContentScore(node, contentScore)
}

// removeAndGetNext remove node and returns its next node.
func (r *Readability) removeAndGetNext(node *html.Node) *html.Node {
	nextNode := r.getNextNode(node, true)

	if node.Parent != nil {
		node.Parent.RemoveChild(node)
	}

	return nextNode
}

// getNextNode traverses the DOM from node to node, starting at the node passed
// in. Pass true for the second parameter to indicate this node itself (and its
// kids) are going away, and we want the next node over. Calling this in a loop
// will traverse the DOM depth-first.
//
// In Readability.js, ignoreSelfAndKids default to false.
func (r *Readability) getNextNode(node *html.Node, ignoreSelfAndKids bool) *html.Node {
	// First check for kids if those are not being ignored
	if firstChild := firstElementChild(node); !ignoreSelfAndKids && firstChild != nil {
		return firstChild
	}

	// Then for siblings...
	if sibling := nextElementSibling(node); sibling != nil {
		return sibling
	}

	// And finally, move up the parent chain *and* find a sibling
	// (because this is depth-first traversal, we will have already
	// seen the parent nodes themselves).
	for {
		node = node.Parent
		if node == nil || nextElementSibling(node) != nil {
			break
		}
	}

	if node != nil {
		return nextElementSibling(node)
	}

	return nil
}

// isValidByline checks whether the input string could be a byline.
func (r *Readability) isValidByline(byline string) bool {
	byline = strings.TrimSpace(byline)
	return len(byline) > 0 && len(byline) < 100
}

// checkByline determines if a node is used as byline.
func (r *Readability) checkByline(node *html.Node, matchString string) bool {
	if r.articleByline != "" {
		return false
	}

	rel := getAttribute(node, "rel")
	itemprop := getAttribute(node, "itemprop")
	nodeText := textContent(node)
	if (rel == "author" || strings.Contains(itemprop, "author") || rxByline.MatchString(matchString)) && r.isValidByline(nodeText) {
		nodeText = strings.TrimSpace(nodeText)
		nodeText = strings.Join(strings.Fields(nodeText), "\x20")
		r.articleByline = nodeText
		return true
	}

	return false
}

// getNodeAncestors gets the node's direct parent and grandparents.
//
// In Readability.js, maxDepth default to 0.
func (r *Readability) getNodeAncestors(node *html.Node, maxDepth int) []*html.Node {
	level := 0
	ancestors := []*html.Node{}

	for node.Parent != nil {
		level++
		ancestors = append(ancestors, node.Parent)

		if maxDepth > 0 && level == maxDepth {
			break
		}

		node = node.Parent
	}

	return ancestors
}

// setContentScore sets the readability score for a node.
func (r *Readability) setContentScore(node *html.Node, score float64) {
	setAttribute(node, "data-readability-score", fmt.Sprintf("%.4f", score))
}

// hasContentScore checks if node has readability score.
func (r *Readability) hasContentScore(node *html.Node) bool {
	return hasAttribute(node, "data-readability-score")
}

// getContentScore gets the readability score of a node.
func (r *Readability) getContentScore(node *html.Node) float64 {
	strScore := getAttribute(node, "data-readability-score")
	strScore = strings.TrimSpace(strScore)

	if strScore == "" {
		return 0
	}

	score, err := strconv.ParseFloat(strScore, 64)

	if err != nil {
		return 0
	}

	return score
}

// removeScripts removes script tags from the document.
func (r *Readability) removeScripts(doc *html.Node) {
	r.removeNodes(getElementsByTagName(doc, "script"), nil)
	r.removeNodes(getElementsByTagName(doc, "noscript"), nil)
}

// hasSingleTagInsideElement check if the node has only whitespace and a single
// element with given tag. Returns false if the DIV Node contains non-empty text
// nodes or if it contains no element with given tag or more than 1 element.
func (r *Readability) hasSingleTagInsideElement(element *html.Node, tag string) bool {
	// There should be exactly 1 element child with given tag
	if childs := children(element); len(childs) != 1 || tagName(childs[0]) != tag {
		return false
	}

	// And there should be no text nodes with real content
	return !r.someNode(childNodes(element), func(node *html.Node) bool {
		return node.Type == html.TextNode && rxHasContent.MatchString(textContent(node))
	})
}

// isElementWithoutContent determines if node is empty. A node is considered
// empty is there is nothing inside or if the only things inside are HTML break
// tags <br> and HTML horizontal rule tags <hr>.
func (r *Readability) isElementWithoutContent(node *html.Node) bool {
	brs := getElementsByTagName(node, "br")
	hrs := getElementsByTagName(node, "hr")
	childs := children(node)

	return node.Type == html.ElementNode &&
		strings.TrimSpace(textContent(node)) == "" &&
		(len(childs) == 0 || len(childs) == len(brs)+len(hrs))
}

// hasChildBlockElement determines whether element has any children block level
// elements.
func (r *Readability) hasChildBlockElement(element *html.Node) bool {
	return r.someNode(childNodes(element), func(node *html.Node) bool {
		return indexOf(divToPElems, tagName(node)) != -1 ||
			r.hasChildBlockElement(node)
	})
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

// getInnerText gets the inner text of a node.
// This also strips out any excess whitespace to be found.
// In Readability.js, normalizeSpaces default to true.
func (r *Readability) getInnerText(node *html.Node, normalizeSpaces bool) string {
	textContent := strings.TrimSpace(textContent(node))

	if normalizeSpaces {
		textContent = rxNormalize.ReplaceAllString(textContent, "\x20")
	}

	return textContent
}

// cleanStyles removes the style attribute on every node and under.
func (r *Readability) cleanStyles(node *html.Node) {
	nodeTagName := tagName(node)

	if node == nil || nodeTagName == "svg" {
		return
	}

	// Remove `style` and deprecated presentational attributes
	for i := 0; i < len(presentationalAttributes); i++ {
		removeAttribute(node, presentationalAttributes[i])
	}

	if indexOf(deprecatedSizeAttributeElems, nodeTagName) != -1 {
		removeAttribute(node, "width")
		removeAttribute(node, "height")
	}

	for child := firstElementChild(node); child != nil; child = nextElementSibling(child) {
		r.cleanStyles(child)
	}
}

// getLinkDensity gets the density of links as a percentage of the content.
// This is the amount of text that is inside a link divided by the total text
// in the node.
func (r *Readability) getLinkDensity(element *html.Node) float64 {
	textLength := len(r.getInnerText(element, true))

	if textLength == 0 {
		return 0
	}

	linkLength := 0

	r.forEachNode(getElementsByTagName(element, "a"), func(linkNode *html.Node, _ int) {
		linkLength += len(r.getInnerText(linkNode, true))
	})

	return float64(linkLength) / float64(textLength)
}

// getClassWeight gets an elements class/id weight. Uses regular expressions to
// tell if this element looks good or bad.
func (r *Readability) getClassWeight(node *html.Node) int {
	if !r.flags.useWeightClasses {
		return 0
	}

	weight := 0

	// Look for a special classname
	if nodeClassName := className(node); nodeClassName != "" {
		if rxNegative.MatchString(nodeClassName) {
			weight -= 25
		}

		if rxPositive.MatchString(nodeClassName) {
			weight += 25
		}
	}

	// Look for a special ID
	if nodeID := id(node); nodeID != "" {
		if rxNegative.MatchString(nodeID) {
			weight -= 25
		}

		if rxPositive.MatchString(nodeID) {
			weight += 25
		}
	}

	return weight
}

// hasAncestorTag checks if a given node has one of its ancestor tag name
// matching the provided one.
//
// In Readability.js, default value for maxDepth is 3.
func (r *Readability) hasAncestorTag(node *html.Node, tag string, maxDepth int, filterFn func(*html.Node) bool) bool {
	depth := 0

	for node.Parent != nil {
		if maxDepth > 0 && depth > maxDepth {
			return false
		}

		if tagName(node.Parent) == tag && (filterFn == nil || filterFn(node.Parent)) {
			return true
		}

		node = node.Parent

		depth++
	}

	return false
}

// getRowAndColumnCount returns how many rows and columns this table has.
func (r *Readability) getRowAndColumnCount(table *html.Node) (int, int) {
	rows := 0
	columns := 0
	trs := getElementsByTagName(table, "tr")

	for i := 0; i < len(trs); i++ {
		strRowSpan := getAttribute(trs[i], "rowspan")
		rowSpan, _ := strconv.Atoi(strRowSpan)

		if rowSpan == 0 {
			rowSpan = 1
		}

		rows += rowSpan

		// Now look for column-related info
		columnsInThisRow := 0
		cells := getElementsByTagName(trs[i], "td")

		for j := 0; j < len(cells); j++ {
			strColSpan := getAttribute(cells[j], "colspan")
			colSpan, _ := strconv.Atoi(strColSpan)

			if colSpan == 0 {
				colSpan = 1
			}

			columnsInThisRow += colSpan
		}

		if columnsInThisRow > columns {
			columns = columnsInThisRow
		}
	}

	return rows, columns
}

// isReadabilityDataTable determines if node is data table.
func (r *Readability) isReadabilityDataTable(node *html.Node) bool {
	return hasAttribute(node, "data-readability-table")
}

// setReadabilityDataTable marks whether a Node is data table or not.
func (r *Readability) setReadabilityDataTable(node *html.Node, isDataTable bool) {
	if isDataTable {
		setAttribute(node, "data-readability-table", "true")
		return
	}

	removeAttribute(node, "data-readability-table")
}

// markDataTables looks for "data" (as opposed to "layout") tables and mark it.
func (r *Readability) markDataTables(root *html.Node) {
	tables := getElementsByTagName(root, "table")

	for i := 0; i < len(tables); i++ {
		table := tables[i]

		role := getAttribute(table, "role")
		if role == "presentation" {
			r.setReadabilityDataTable(table, false)
			continue
		}

		datatable := getAttribute(table, "datatable")
		if datatable == "0" {
			r.setReadabilityDataTable(table, false)
			continue
		}

		if hasAttribute(table, "summary") {
			r.setReadabilityDataTable(table, true)
			continue
		}

		if captions := getElementsByTagName(table, "caption"); len(captions) > 0 {
			if caption := captions[0]; caption != nil && len(childNodes(caption)) > 0 {
				r.setReadabilityDataTable(table, true)
				continue
			}
		}

		// If the table has a descendant with any of these tags, consider a data table:
		hasDataTableDescendantTags := false
		for _, descendantTag := range []string{"col", "colgroup", "tfoot", "thead", "th"} {
			descendants := getElementsByTagName(table, descendantTag)
			if len(descendants) > 0 && descendants[0] != nil {
				hasDataTableDescendantTags = true
				break
			}
		}

		if hasDataTableDescendantTags {
			r.setReadabilityDataTable(table, true)
			continue
		}

		// Nested tables indicates a layout table:
		if len(getElementsByTagName(table, "table")) > 0 {
			r.setReadabilityDataTable(table, false)
			continue
		}

		rows, columns := r.getRowAndColumnCount(table)

		if rows >= 10 || columns > 4 {
			r.setReadabilityDataTable(table, true)
			continue
		}

		// Now just go by size entirely:
		if rows*columns > 10 {
			r.setReadabilityDataTable(table, true)
		}
	}
}

// isProbablyVisible determines if a node is visible.
func (r *Readability) isProbablyVisible(node *html.Node) bool {
	style := getAttribute(node, "style")
	noStyle := (style == "" || !rxDisplayNone.MatchString(style))
	return noStyle && !hasAttribute(node, "hidden")
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
