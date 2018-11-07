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

func New(reader io.Reader, rawurl string) (*Readability, error) {
	var err error

	r := new(Readability)

	if r.uri, err = url.ParseRequestURI(rawurl); err != nil {
		return nil, fmt.Errorf("url.ParseRequestURI %s", err)
	}

	if r.doc, err = html.Parse(reader); err != nil {
		return nil, fmt.Errorf("html.Parse %s", err)
	}

	return r, nil
}

// Parse runs readability.
func (r *Readability) Parse() (Article, error) {
	if r.MaxElemsToParse > 0 {
		numTags := len(getElementsByTagName(r.doc, "*"))
		if numTags > r.MaxElemsToParse {
			return Article{}, fmt.Errorf("aborting parsing document; %d elements found", numTags)
		}
	}

	return Article{}, nil
}
