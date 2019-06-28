package readability

import (
	"fmt"
	"io"
	"math"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/net/html"
)

// All of the regular expressions in use within readability.
// Defined up here so we don't instantiate them repeatedly in loops.
var rxUnlikelyCandidates = regexp.MustCompile(`(?i)-ad-|ai2html|banner|breadcrumbs|combx|comment|community|cover-wrap|disqus|extra|foot|gdpr|header|legends|menu|related|remark|replies|rss|shoutbox|sidebar|skyscraper|social|sponsor|supplemental|ad-break|agegate|pagination|pager|popup|yom-remote`)
var rxOkMaybeItsACandidate = regexp.MustCompile(`(?i)and|article|body|column|main|shadow`)
var rxPositive = regexp.MustCompile(`(?i)article|body|content|entry|hentry|h-entry|main|page|pagination|post|text|blog|story`)
var rxNegative = regexp.MustCompile(`(?i)hidden|^hid$| hid$| hid |^hid |banner|combx|comment|com-|contact|foot|footer|footnote|gdpr|masthead|media|meta|outbrain|promo|related|scroll|share|shoutbox|sidebar|skyscraper|sponsor|shopping|tags|tool|widget`)
var rxByline = regexp.MustCompile(`(?i)byline|author|dateline|writtenby|p-author`)
var rxNormalize = regexp.MustCompile(`(?i)\s{2,}`)
var rxVideos = regexp.MustCompile(`(?i)//(www\.)?((dailymotion|youtube|youtube-nocookie|player\.vimeo|v\.qq)\.com|(archive|upload\.wikimedia)\.org|player\.twitch\.tv)`)
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
var rxSentencePeriod = regexp.MustCompile(`(?i)\.( |$)`)
var rxShare = regexp.MustCompile(`(?i)share`)
var rxFaviconSize = regexp.MustCompile(`(?i)(\d+)x(\d+)`)

// divToPElems is a list of HTML tag names representing content dividers.
var divToPElems = []string{
	"a", "blockquote", "div", "dl", "img",
	"ol", "p", "pre", "select", "table", "ul",
}

// alterToDivExceptions is a list of HTML tags that we want to convert into
// regular DIV elements to prevent unwanted removal when the parser is cleaning
// out unnecessary Nodes.
var alterToDivExceptions = []string{
	"article",
	"div",
	"p",
	"section",
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
	stripUnlikelys     bool
	useWeightClasses   bool
	cleanConditionally bool
}

// parseAttempt is container for the result of previous parse attempts.
type parseAttempt struct {
	articleContent *html.Node
	textLength     int
}

// Readability is an HTML parser that reads and extract relevant content.
type Readability struct {
	doc           *html.Node
	documentURI   *url.URL
	articleTitle  string
	articleByline string
	attempts      []parseAttempt
	flags         flags

	// MaxElemsToParse is the optional maximum number of HTML nodes to parse
	// from the document. If the number of elements in the document is higher
	// than this number, the operation immediately errors.
	MaxElemsToParse int

	// NTopCandidates is the number of top candidates to consider when the
	// parser is analysing how tight the competition is among candidates.
	NTopCandidates int

	// CharThresholds is the default number of chars an article must have in
	// order to return a result.
	CharThresholds int

	// ClassesToPreserve are the classes that readability sets itself.
	ClassesToPreserve []string

	// TagsToScore is element tags to score by default.
	TagsToScore []string
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

	// Node is the first element in the HTML document.
	Node *html.Node
}

// New returns new Readability with sane defaults to parse simple documents.
func New() *Readability {
	return &Readability{
		MaxElemsToParse:   0,
		NTopCandidates:    5,
		CharThresholds:    500,
		ClassesToPreserve: []string{"page"},
		TagsToScore:       []string{"section", "h2", "h3", "h4", "h5", "h6", "p", "td", "pre"},
	}
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
		r.setNodeTag(list[i], newTagName)
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
	curTitle = rxNormalize.ReplaceAllString(curTitle, "\x20")
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

// prepArticle prepares the article Node for display cleaning out any inline
// CSS styles, iframes, forms and stripping extraneous paragraph tags <p>.
func (r *Readability) prepArticle(articleContent *html.Node) {
	r.cleanStyles(articleContent)

	// Check for data tables before we continue, to avoid removing
	// items in those tables, which will often be isolated even
	// though they're visually linked to other content-ful elements
	// (text, images, etc.).
	r.markDataTables(articleContent)

	// Clean out junk from the article content
	r.cleanConditionally(articleContent, "form")
	r.cleanConditionally(articleContent, "fieldset")
	r.clean(articleContent, "object")
	r.clean(articleContent, "embed")
	r.clean(articleContent, "h1")
	r.clean(articleContent, "footer")
	r.clean(articleContent, "link")
	r.clean(articleContent, "aside")

	// Clean out elements have "share" in their id/class combinations
	// from final top candidates, which means we don't remove the top
	// candidates even they have "share".
	r.forEachNode(children(articleContent), func(topCandidate *html.Node, _ int) {
		r.cleanMatchedNodes(topCandidate, func(node *html.Node, nodeClassID string) bool {
			return rxShare.MatchString(nodeClassID) && len(textContent(node)) < r.CharThresholds
		})
	})

	// If there is only one h2 and its text content substantially
	// equals article title, they are probably using it as a header
	// and not a subheader, so remove it since we already extract
	// the title separately.
	if h2s := getElementsByTagName(articleContent, "h2"); len(h2s) == 1 {
		h2 := h2s[0]
		h2Text := textContent(h2)
		lengthSimilarRate := float64(len(h2Text)-len(r.articleTitle)) / float64(len(r.articleTitle))

		if math.Abs(lengthSimilarRate) < 0.5 {
			titlesMatch := false

			if lengthSimilarRate > 0 {
				titlesMatch = strings.Contains(h2Text, r.articleTitle)
			} else {
				titlesMatch = strings.Contains(r.articleTitle, h2Text)
			}

			if titlesMatch {
				r.clean(articleContent, "h2")
			}
		}
	}

	r.clean(articleContent, "iframe")
	r.clean(articleContent, "input")
	r.clean(articleContent, "textarea")
	r.clean(articleContent, "select")
	r.clean(articleContent, "button")
	r.cleanHeaders(articleContent)

	// Do these last as the previous stuff may have removed junk
	// that will affect these
	r.cleanConditionally(articleContent, "table")
	r.cleanConditionally(articleContent, "ul")
	r.cleanConditionally(articleContent, "div")

	// Remove extra paragraphs
	r.removeNodes(getElementsByTagName(articleContent, "p"), func(p *html.Node) bool {
		imgCount := len(getElementsByTagName(p, "img"))
		embedCount := len(getElementsByTagName(p, "embed"))
		objectCount := len(getElementsByTagName(p, "object"))

		// Nasty iframes have been removed, only remain embedded videos.
		iframeCount := len(getElementsByTagName(p, "iframe"))
		totalCount := imgCount + embedCount + objectCount + iframeCount

		return totalCount == 0 && r.getInnerText(p, false) == ""
	})

	r.forEachNode(getElementsByTagName(articleContent, "br"), func(br *html.Node, _ int) {
		next := r.nextElement(br.NextSibling)

		if next != nil && tagName(next) == "p" {
			br.Parent.RemoveChild(br)
		}
	})

	// Remove single-cell tables
	r.forEachNode(getElementsByTagName(articleContent, "table"), func(table *html.Node, _ int) {
		tbody := table

		if r.hasSingleTagInsideElement(table, "tbody") {
			tbody = firstElementChild(table)
		}

		if r.hasSingleTagInsideElement(tbody, "tr") {
			row := firstElementChild(tbody)

			if r.hasSingleTagInsideElement(row, "td") {
				cell := firstElementChild(row)

				newTag := "div"

				if r.everyNode(childNodes(cell), r.isPhrasingContent) {
					newTag = "p"
				}

				r.setNodeTag(cell, newTag)

				replaceNode(table, cell)
			}
		}
	})
}

// grabArticle uses a variety of metrics (content score, classname, element
// types), find the content that is most likely to be the stuff a user wants to
// read. Then return it wrapped up in a div.
func (r *Readability) grabArticle() *html.Node {
	for {
		doc := cloneNode(r.doc)

		var page *html.Node
		if nodes := getElementsByTagName(doc, "body"); len(nodes) > 0 {
			page = nodes[0]
		}

		// We can not grab an article if we do not have a page.
		if page == nil {
			return nil
		}

		// First, node prepping. Trash nodes that look cruddy (like ones with
		// the class name "comment", etc), and turn divs into P tags where they
		// have been used inappropriately (as in, where they contain no other
		// block level elements).
		var elementsToScore []*html.Node
		var node = documentElement(doc)

		for node != nil {
			matchString := className(node) + "\x20" + id(node)

			if !r.isProbablyVisible(node) {
				node = r.removeAndGetNext(node)
				continue
			}

			// Remove Node if it is a Byline.
			if r.checkByline(node, matchString) {
				node = r.removeAndGetNext(node)
				continue
			}

			// Remove unlikely candidates.
			nodeTagName := tagName(node)
			if r.flags.stripUnlikelys {
				if rxUnlikelyCandidates.MatchString(matchString) &&
					!rxOkMaybeItsACandidate.MatchString(matchString) &&
					!r.hasAncestorTag(node, "table", 3, nil) &&
					nodeTagName != "body" &&
					nodeTagName != "a" {
					node = r.removeAndGetNext(node)
					continue
				}
			}

			// Remove DIV, SECTION and HEADER nodes without any content.
			switch nodeTagName {
			case "div",
				"section",
				"header",
				"h1",
				"h2",
				"h3",
				"h4",
				"h5",
				"h6":
				if r.isElementWithoutContent(node) {
					node = r.removeAndGetNext(node)
					continue
				}
			}

			if indexOf(r.TagsToScore, nodeTagName) != -1 {
				elementsToScore = append(elementsToScore, node)
			}

			// Convert <div> without children block level elements into <p>.
			if nodeTagName == "div" {
				// Put phrasing content into paragraphs.
				var p *html.Node
				childNode := node.FirstChild

				for childNode != nil {
					nextSibling := childNode.NextSibling

					if r.isPhrasingContent(childNode) {
						if p != nil {
							appendChild(p, childNode)
						} else if !r.isWhitespace(childNode) {
							p = createElement("p")
							appendChild(p, cloneNode(childNode))
							replaceNode(childNode, p)
						}
					} else if p != nil {
						for p.LastChild != nil && r.isWhitespace(p.LastChild) {
							p.RemoveChild(p.LastChild)
						}
						p = nil
					}

					childNode = nextSibling
				}

				// Sites like http://mobile.slate.com encloses each paragraph
				// with a DIV element. DIVs with only a P element inside and no
				// text content can be safely converted into plain P elements to
				// avoid confusing the scoring algorithm with DIVs with are, in
				// practice, paragraphs.
				if r.hasSingleTagInsideElement(node, "p") && r.getLinkDensity(node) < 0.25 {
					newNode := children(node)[0]
					replaceNode(node, newNode)
					node = newNode
					elementsToScore = append(elementsToScore, node)
				} else if !r.hasChildBlockElement(node) {
					r.setNodeTag(node, "p")
					elementsToScore = append(elementsToScore, node)
				}
			}

			node = r.getNextNode(node, false)
		}

		// Loop through all paragraphs and assign a score to them based on how
		// much relevant content they have. Then add their score to their parent
		// node. A score is determined by things like number of commas, class
		// names, etc. Maybe eventually link density.
		var candidates []*html.Node
		r.forEachNode(elementsToScore, func(elementToScore *html.Node, _ int) {
			if elementToScore.Parent == nil || tagName(elementToScore.Parent) == "" {
				return
			}

			// If this paragraph is less than 25 characters, don't even count it.
			innerText := r.getInnerText(elementToScore, true)
			if len(innerText) < 25 {
				return
			}

			// Exclude nodes with no ancestor.
			ancestors := r.getNodeAncestors(elementToScore, 3)
			if len(ancestors) == 0 {
				return
			}

			// Add a point for the paragraph itself as a base.
			contentScore := 1

			// Add points for any commas within this paragraph.
			contentScore += strings.Count(innerText, ",")

			// For every 100 characters in this paragraph, add another point. Up to 3 points.
			contentScore += int(math.Min(math.Floor(float64(len(innerText))/100.0), 3.0))

			// Initialize and score ancestors.
			r.forEachNode(ancestors, func(ancestor *html.Node, level int) {
				if tagName(ancestor) == "" || ancestor.Parent == nil || ancestor.Parent.Type != html.ElementNode {
					return
				}

				if !r.hasContentScore(ancestor) {
					r.initializeNode(ancestor)
					candidates = append(candidates, ancestor)
				}

				// Node score divider:
				// - parent:             1 (no division)
				// - grandparent:        2
				// - great grandparent+: ancestor level * 3
				scoreDivider := 1
				switch level {
				case 0:
					scoreDivider = 1
				case 1:
					scoreDivider = 2
				default:
					scoreDivider = level * 3
				}

				ancestorScore := r.getContentScore(ancestor)
				ancestorScore += float64(contentScore) / float64(scoreDivider)

				r.setContentScore(ancestor, ancestorScore)
			})
		})

		// These lines are a bit different compared to Readability.js.
		//
		// In Readability.js, they fetch NTopCandidates utilising array method
		// like `splice` and `pop`. In Go, array method like that is not as
		// simple, especially since we are working with pointer. So, here we
		// simply sort top candidates, and limit it to max NTopCandidates.

		// Scale the final candidates score based on link density. Good
		// content should have a relatively small link density (5% or
		// less) and be mostly unaffected by this operation.
		for i := 0; i < len(candidates); i++ {
			candidate := candidates[i]
			candidateScore := r.getContentScore(candidate) * (1 - r.getLinkDensity(candidate))
			r.setContentScore(candidate, candidateScore)
		}

		// After we have calculated scores, sort through all of the possible
		// candidate nodes we found and find the one with the highest score.
		sort.Slice(candidates, func(i int, j int) bool {
			return r.getContentScore(candidates[i]) > r.getContentScore(candidates[j])
		})

		var topCandidates []*html.Node

		if len(candidates) > r.NTopCandidates {
			topCandidates = candidates[:r.NTopCandidates]
		} else {
			topCandidates = candidates
		}

		var topCandidate, parentOfTopCandidate *html.Node
		neededToCreateTopCandidate := false
		if len(topCandidates) > 0 {
			topCandidate = topCandidates[0]
		}

		// If we still have no top candidate, just use the body as a last
		// resort. We also have to copy the body node so it is something
		// we can modify.
		if topCandidate == nil || tagName(topCandidate) == "body" {
			// Move all of the page's children into topCandidate
			topCandidate = createElement("div")
			neededToCreateTopCandidate = true
			// Move everything (not just elements, also text nodes etc.)
			// into the container so we even include text directly in the body:
			kids := childNodes(page)
			for i := 0; i < len(kids); i++ {
				appendChild(topCandidate, kids[i])
			}

			appendChild(page, topCandidate)
			r.initializeNode(topCandidate)
		} else if topCandidate != nil {
			// Find a better top candidate node if it contains (at least three)
			// nodes which belong to `topCandidates` array and whose scores are
			// quite closed with current `topCandidate` node.
			topCandidateScore := r.getContentScore(topCandidate)
			var alternativeCandidateAncestors [][]*html.Node
			for i := 1; i < len(topCandidates); i++ {
				if r.getContentScore(topCandidates[i])/topCandidateScore >= 0.75 {
					topCandidateAncestors := r.getNodeAncestors(topCandidates[i], 0)
					alternativeCandidateAncestors = append(alternativeCandidateAncestors, topCandidateAncestors)
				}
			}

			minimumTopCandidates := 3
			if len(alternativeCandidateAncestors) >= minimumTopCandidates {
				parentOfTopCandidate = topCandidate.Parent
				for parentOfTopCandidate != nil && tagName(parentOfTopCandidate) != "body" {
					listContainingThisAncestor := 0
					for ancestorIndex := 0; ancestorIndex < len(alternativeCandidateAncestors) && listContainingThisAncestor < minimumTopCandidates; ancestorIndex++ {
						if includeNode(alternativeCandidateAncestors[ancestorIndex], parentOfTopCandidate) {
							listContainingThisAncestor++
						}
					}

					if listContainingThisAncestor >= minimumTopCandidates {
						topCandidate = parentOfTopCandidate
						break
					}

					parentOfTopCandidate = parentOfTopCandidate.Parent
				}
			}

			if !r.hasContentScore(topCandidate) {
				r.initializeNode(topCandidate)
			}

			// Because of our bonus system, parents of candidates might
			// have scores themselves. They get half of the node. There
			// won't be nodes with higher scores than our topCandidate,
			// but if we see the score going *up* in the first few steps *
			// up the tree, that's a decent sign that there might be more
			// content lurking in other places that we want to unify in.
			// The sibling stuff below does some of that - but only if
			// we've looked high enough up the DOM tree.
			parentOfTopCandidate = topCandidate.Parent
			lastScore := r.getContentScore(topCandidate)
			// The scores shouldn't get too lor.
			scoreThreshold := lastScore / 3.0
			for parentOfTopCandidate != nil && tagName(parentOfTopCandidate) != "body" {
				if !r.hasContentScore(parentOfTopCandidate) {
					parentOfTopCandidate = parentOfTopCandidate.Parent
					continue
				}

				parentScore := r.getContentScore(parentOfTopCandidate)
				if parentScore < scoreThreshold {
					break
				}

				if parentScore > lastScore {
					// Alright! We found a better parent to use.
					topCandidate = parentOfTopCandidate
					break
				}

				lastScore = parentScore
				parentOfTopCandidate = parentOfTopCandidate.Parent
			}

			// If the top candidate is the only child, use parent
			// instead. This will help sibling joining logic when
			// adjacent content is actually located in parent's
			// sibling node.
			parentOfTopCandidate = topCandidate.Parent
			for parentOfTopCandidate != nil && tagName(parentOfTopCandidate) != "body" && len(children(parentOfTopCandidate)) == 1 {
				topCandidate = parentOfTopCandidate
				parentOfTopCandidate = topCandidate.Parent
			}

			if !r.hasContentScore(topCandidate) {
				r.initializeNode(topCandidate)
			}
		}

		// Now that we have the top candidate, look through its siblings
		// for content that might also be related. Things like preambles,
		// content split by ads that we removed, etc.
		articleContent := createElement("div")
		siblingScoreThreshold := math.Max(10, r.getContentScore(topCandidate)*0.2)

		// Keep potential top candidate's parent node to try to get text direction of it later.
		topCandidateScore := r.getContentScore(topCandidate)
		topCandidateClassName := className(topCandidate)

		parentOfTopCandidate = topCandidate.Parent
		siblings := children(parentOfTopCandidate)
		for s := 0; s < len(siblings); s++ {
			sibling := siblings[s]
			appendNode := false

			if sibling == topCandidate {
				appendNode = true
			} else {
				contentBonus := float64(0)

				// Give a bonus if sibling nodes and top candidates have the example same classname
				if className(sibling) == topCandidateClassName && topCandidateClassName != "" {
					contentBonus += topCandidateScore * 0.2
				}

				if r.hasContentScore(sibling) && r.getContentScore(sibling)+contentBonus >= siblingScoreThreshold {
					appendNode = true
				} else if tagName(sibling) == "p" {
					linkDensity := r.getLinkDensity(sibling)
					nodeContent := r.getInnerText(sibling, true)
					nodeLength := len(nodeContent)

					if nodeLength > 80 && linkDensity < 0.25 {
						appendNode = true
					} else if nodeLength < 80 && nodeLength > 0 && linkDensity == 0 &&
						rxSentencePeriod.MatchString(nodeContent) {
						appendNode = true
					}
				}
			}

			if appendNode {
				// We have a node that is not a common block level element,
				// like a FORM or TD tag. Turn it into a DIV so it does not get
				// filtered out later by accident.
				if indexOf(alterToDivExceptions, tagName(sibling)) == -1 {
					r.setNodeTag(sibling, "div")
				}

				appendChild(articleContent, sibling)
			}
		}

		// So we have all of the content that we need. Now we clean
		// it up for presentation.
		r.prepArticle(articleContent)

		if neededToCreateTopCandidate {
			// We already created a fake DIV thing, and there would not have
			// been any siblings left for the previous loop, so there is no
			// point trying to create a new DIV and then move all the children
			// over. Just assign IDs and CSS class names here. No need to append
			// because that already happened anyway.
			//
			// By the way, this line is different with Readability.js.
			//
			// In Readability.js, when using `appendChild`, the node is still
			// referenced. Meanwhile here, our `appendChild` will clone the
			// node, put it in the new place, then delete the original.
			firstChild := firstElementChild(articleContent)
			if firstChild != nil && tagName(firstChild) == "div" {
				setAttribute(firstChild, "id", "readability-page-1")
				setAttribute(firstChild, "class", "page")
			}
		} else {
			div := createElement("div")

			setAttribute(div, "id", "readability-page-1")
			setAttribute(div, "class", "page")

			childs := childNodes(articleContent)

			for i := 0; i < len(childs); i++ {
				appendChild(div, childs[i])
			}

			appendChild(articleContent, div)
		}

		parseSuccessful := true

		// Now that we've gone through the full algorithm, check to see if we
		// got any meaningful content. If we did not, we may need to re-run
		// grabArticle with different flags set. This gives us a higher
		// likelihood of finding the content, and the sieve approach gives us a
		// higher likelihood of finding the -right- content.
		textLength := len(r.getInnerText(articleContent, true))
		if textLength < r.CharThresholds {
			parseSuccessful = false

			if r.flags.stripUnlikelys {
				r.flags.stripUnlikelys = false
				r.attempts = append(r.attempts, parseAttempt{
					articleContent: articleContent,
					textLength:     textLength,
				})
			} else if r.flags.useWeightClasses {
				r.flags.useWeightClasses = false
				r.attempts = append(r.attempts, parseAttempt{
					articleContent: articleContent,
					textLength:     textLength,
				})
			} else if r.flags.cleanConditionally {
				r.flags.cleanConditionally = false
				r.attempts = append(r.attempts, parseAttempt{
					articleContent: articleContent,
					textLength:     textLength,
				})
			} else {
				r.attempts = append(r.attempts, parseAttempt{
					articleContent: articleContent,
					textLength:     textLength,
				})

				// No luck after removing flags, just return the
				// longest text we found during the different loops *
				sort.Slice(r.attempts, func(i, j int) bool {
					return r.attempts[i].textLength > r.attempts[j].textLength
				})

				// But first check if we actually have something
				if r.attempts[0].textLength == 0 {
					return nil
				}

				articleContent = r.attempts[0].articleContent
				parseSuccessful = true
			}
		}

		if parseSuccessful {
			return articleContent
		}
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

// getCharCount returns the number of times a string appears in the Node.
func (r *Readability) getCharCount(node *html.Node, s string) int {
	innerText := r.getInnerText(node, true)
	return strings.Count(innerText, s)
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

// clean cleans a node of all elements of type "tag".
func (r *Readability) clean(node *html.Node, tag string) {
	isEmbed := indexOf([]string{"object", "embed", "iframe"}, tag) != -1

	r.removeNodes(getElementsByTagName(node, tag), func(element *html.Node) bool {
		// Allow YouTube and Vimeo videos through as people usually want to see those.
		if isEmbed {
			// Check the attributes to see if any of them contain YouTube or Vimeo.
			for _, attr := range element.Attr {
				if rxVideos.MatchString(attr.Val) {
					return false
				}
			}

			// For embed with <object> tag, check inner HTML as well.
			if tagName(element) == "object" && rxVideos.MatchString(innerHTML(element)) {
				return false
			}
		}

		return true
	})
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

// isReadabilityDataTable determines if a Node is a data table.
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

// cleanConditionally cleans an element of all tags of type "tag" if they look
// fishy. "Fishy" is an algorithm based on content length, classnames, link
// density, number of images & embeds, etc.
func (r *Readability) cleanConditionally(element *html.Node, tag string) {
	if !r.flags.cleanConditionally {
		return
	}

	isList := tag == "ul" || tag == "ol"

	// Gather counts for other typical elements embedded within. Traverse
	// backwards so we can remove nodes at the same time without effecting
	// the traversal.
	r.removeNodes(getElementsByTagName(element, tag), func(node *html.Node) bool {
		if tag == "table" && r.isReadabilityDataTable(node) {
			return false
		}

		if r.hasAncestorTag(node, "table", -1, r.isReadabilityDataTable) {
			return false
		}

		weight := r.getClassWeight(node)
		if weight < 0 {
			return true
		}

		if r.getCharCount(node, ",") < 10 {
			// If there are not many commas and the number of non-paragraph
			// elements is more than paragraphs or other ominous signs, remove
			// the element.
			p := float64(len(getElementsByTagName(node, "p")))
			img := float64(len(getElementsByTagName(node, "img")))
			li := float64(len(getElementsByTagName(node, "li")) - 100)
			input := float64(len(getElementsByTagName(node, "input")))

			embedCount := 0
			embeds := r.concatNodeLists(
				getElementsByTagName(node, "object"),
				getElementsByTagName(node, "embed"),
				getElementsByTagName(node, "iframe"),
			)

			for _, embed := range embeds {
				// Do not delete if Embed has attribute matching Video regex.
				for _, attr := range embed.Attr {
					if rxVideos.MatchString(attr.Val) {
						return false
					}
				}

				// For embed with <object> tag, check inner HTML as well.
				if tagName(embed) == "object" && rxVideos.MatchString(innerHTML(embed)) {
					return false
				}

				embedCount++
			}

			linkDensity := r.getLinkDensity(node)
			contentLength := len(r.getInnerText(node, true))

			return (img > 1 && p/img < 0.5 && !r.hasAncestorTag(node, "figure", 3, nil)) ||
				(!isList && li > p) ||
				(input > math.Floor(p/3)) ||
				(!isList && contentLength < 25 && (img == 0 || img > 2) && !r.hasAncestorTag(node, "figure", 3, nil)) ||
				(!isList && weight < 25 && linkDensity > 0.2) ||
				(weight >= 25 && linkDensity > 0.5) ||
				((embedCount == 1 && contentLength < 75) || embedCount > 1)
		}

		return false
	})
}

// cleanMatchedNodes cleans out elements whose ID and CSS class combinations
// match specific string.
func (r *Readability) cleanMatchedNodes(e *html.Node, filter func(*html.Node, string) bool) {
	endOfSearchMarkerNode := r.getNextNode(e, true)
	next := r.getNextNode(e, false)

	for next != nil && next != endOfSearchMarkerNode {
		if filter != nil && filter(next, className(next)+"\x20"+id(next)) {
			next = r.removeAndGetNext(next)
		} else {
			next = r.getNextNode(next, false)
		}
	}
}

// cleanHeaders cleans out spurious headers from an Element. Checks things like
// classnames and link density.
func (r *Readability) cleanHeaders(e *html.Node) {
	for headerIndex := 1; headerIndex < 3; headerIndex++ {
		headerTag := fmt.Sprintf("h%d", headerIndex)

		r.removeNodes(getElementsByTagName(e, headerTag), func(header *html.Node) bool {
			return r.getClassWeight(header) < 0
		})
	}
}

// isProbablyVisible determines if a node is visible.
func (r *Readability) isProbablyVisible(node *html.Node) bool {
	style := getAttribute(node, "style")
	noStyle := (style == "" || !rxDisplayNone.MatchString(style))
	return noStyle && !hasAttribute(node, "hidden")
}

// fixRelativeURIs converts each <a> and <img> uri in the given element to an
// absolute URI, ignoring #ref URIs.
func (r *Readability) fixRelativeURIs(articleContent *html.Node) {
	links := r.getAllNodesWithTag(articleContent, "a")

	r.forEachNode(links, func(link *html.Node, _ int) {
		href := getAttribute(link, "href")

		if href == "" {
			return
		}

		// Replace links with javascript: URIs with text content, since they
		// will not work after scripts have been removed from the page.
		if strings.HasPrefix(href, "javascript:") {
			text := createTextNode(textContent(link))
			replaceNode(link, text)
			return
		}

		newHref := toAbsoluteURI(href, r.documentURI)

		if newHref == "" {
			removeAttribute(link, "href")
			return
		}

		setAttribute(link, "href", newHref)
	})

	imgs := r.getAllNodesWithTag(articleContent, "img")

	r.forEachNode(imgs, func(img *html.Node, _ int) {
		src := getAttribute(img, "src")

		if src == "" {
			return
		}

		newSrc := toAbsoluteURI(src, r.documentURI)

		if newSrc == "" {
			removeAttribute(img, "src")
			return
		}

		setAttribute(img, "src", newSrc)
	})
}

// cleanClasses removes the class="" attribute from every element in the given
// subtree, except those that match CLASSES_TO_PRESERVE and classesToPreserve
// array from the options object.
func (r *Readability) cleanClasses(node *html.Node) {
	nodeClassName := className(node)
	preservedClassName := []string{}

	for _, class := range strings.Fields(nodeClassName) {
		if indexOf(r.ClassesToPreserve, class) != -1 {
			preservedClassName = append(preservedClassName, class)
		}
	}

	if len(preservedClassName) > 0 {
		setAttribute(node, "class", strings.Join(preservedClassName, "\x20"))
	} else {
		removeAttribute(node, "class")
	}

	for child := firstElementChild(node); child != nil; child = nextElementSibling(child) {
		r.cleanClasses(child)
	}
}

// clearReadabilityAttr removes Readability attribute created by the parser.
func (r *Readability) clearReadabilityAttr(node *html.Node) {
	removeAttribute(node, "data-readability-score")
	removeAttribute(node, "data-readability-table")

	for child := firstElementChild(node); child != nil; child = nextElementSibling(child) {
		r.clearReadabilityAttr(child)
	}
}

// postProcessContent runs post-process modifications to the article content.
func (r *Readability) postProcessContent(articleContent *html.Node) {
	// Convert relative URIs to absolute URIs so we can open them.
	r.fixRelativeURIs(articleContent)

	// Remove CSS classes.
	r.cleanClasses(articleContent)

	// Remove readability attributes.
	r.clearReadabilityAttr(articleContent)
}

// Parse parses input and find the main readable content.
func (r *Readability) Parse(input io.Reader, pageURL string) (Article, error) {
	var err error

	// Reset parser data
	r.articleTitle = ""
	r.articleByline = ""
	r.attempts = []parseAttempt{}
	r.flags.stripUnlikelys = true
	r.flags.useWeightClasses = true
	r.flags.cleanConditionally = true

	// Parse page URL.
	if r.documentURI, err = url.ParseRequestURI(pageURL); err != nil {
		return Article{}, fmt.Errorf("failed to parse URL: %v", err)
	}

	// Parse input.
	if r.doc, err = html.Parse(input); err != nil {
		return Article{}, fmt.Errorf("failed to parse input: %v", err)
	}

	// Avoid parsing too large documents, as per configuration option.
	if r.MaxElemsToParse > 0 {
		numTags := len(getElementsByTagName(r.doc, "*"))

		if numTags > r.MaxElemsToParse {
			return Article{}, fmt.Errorf("too many elements: %d", numTags)
		}
	}

	// Remove script tags from the document.
	r.removeScripts(r.doc)

	// Prepares the HTML document.
	r.prepDocument()

	// Fetch metadata.
	metadata := r.getArticleMetadata()
	r.articleTitle = metadata.Title

	// Try to grab article content.
	finalHTMLContent := ""
	finalTextContent := ""
	readableNode := &html.Node{}
	articleContent := r.grabArticle()

	if articleContent != nil {
		r.postProcessContent(articleContent)

		// If we have not found an excerpt in the article's metadata, use the
		// article's first paragraph as the excerpt. This is used for displaying
		// a preview of the article's content.
		if metadata.Excerpt == "" {
			paragraphs := getElementsByTagName(articleContent, "p")

			if len(paragraphs) > 0 {
				metadata.Excerpt = strings.TrimSpace(textContent(paragraphs[0]))
			}
		}

		readableNode = firstElementChild(articleContent)
		finalHTMLContent = innerHTML(articleContent)
		finalTextContent = textContent(articleContent)
		finalTextContent = strings.TrimSpace(finalTextContent)
	}

	finalByline := metadata.Byline

	if finalByline == "" {
		finalByline = r.articleByline
	}

	return Article{
		Title:       r.articleTitle,
		Byline:      finalByline,
		Node:        readableNode,
		Content:     finalHTMLContent,
		TextContent: finalTextContent,
		Length:      len(finalTextContent),
		Excerpt:     metadata.Excerpt,
		SiteName:    metadata.SiteName,
		Image:       metadata.Image,
		Favicon:     metadata.Favicon,
	}, nil
}

// IsReadable decides whether the document is usable or not without parsing the
// whole thing. In the original `mozilla/readability` library, this method is
// located in `Readability-readable.js`.
func (r *Readability) IsReadable(input io.Reader) bool {
	doc, err := html.Parse(input)

	if err != nil {
		return false
	}

	// Get <p> and <pre> nodes. Also get DIV nodes which have BR node(s) and
	// append them into the `nodes` variable. Some articles' DOM structures
	// might look like:
	//
	// <div>
	//     Sentences<br>
	//     <br>
	//     Sentences<br>
	// </div>
	//
	// So we need to make sure only fetch the div once.
	// To do so, we will use map as dictionary.
	nodeList := make([]*html.Node, 0)
	nodeDict := make(map[*html.Node]struct{})
	var finder func(*html.Node)

	finder = func(node *html.Node) {
		if node.Type == html.ElementNode {
			tag := tagName(node)
			if tag == "p" || tag == "pre" {
				if _, exist := nodeDict[node]; !exist {
					nodeList = append(nodeList, node)
					nodeDict[node] = struct{}{}
				}
			} else if tag == "br" && node.Parent != nil && tagName(node.Parent) == "div" {
				if _, exist := nodeDict[node.Parent]; !exist {
					nodeList = append(nodeList, node.Parent)
					nodeDict[node.Parent] = struct{}{}
				}
			}
		}

		for child := node.FirstChild; child != nil; child = child.NextSibling {
			finder(child)
		}
	}

	finder(doc)

	// This is a little cheeky, we use the accumulator 'score' to decide what
	// to return from this callback.
	score := float64(0)

	return r.someNode(nodeList, func(node *html.Node) bool {
		if !r.isProbablyVisible(node) {
			return false
		}

		matchString := className(node) + "\x20" + id(node)
		if rxUnlikelyCandidates.MatchString(matchString) &&
			!rxOkMaybeItsACandidate.MatchString(matchString) {
			return false
		}

		if tagName(node) == "p" && r.hasAncestorTag(node, "li", -1, nil) {
			return false
		}

		nodeText := strings.TrimSpace(textContent(node))
		nodeTextLength := len(nodeText)
		if nodeTextLength < 140 {
			return false
		}

		score += math.Sqrt(float64(nodeTextLength - 140))
		if score > 20 {
			return true
		}

		return false
	})
}
