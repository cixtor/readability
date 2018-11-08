package readability_test

import (
	"strings"
	"testing"

	"github.com/cixtor/readability"
)

func TestMaxElemsToParse(t *testing.T) {
	var err error
	var r readability.Readability

	input := strings.NewReader(`<html>
		<head>
			<title>hello world</title>
		</head>
		<body>
			<p>lorem ipsum</p>
		</body>
		</html>`)

	if r, err = readability.New(input, "https://cixtor.com/blog"); err != nil {
		t.Fatalf("not initialized: %s", err)
	}

	r.MaxElemsToParse = 3

	if _, err = r.Parse(); err == nil {
		t.Fatalf("MaxElemsToParse should fail if document is too long")
	}
}

func TestRemoveScripts(t *testing.T) {
	var err error
	var a readability.Article
	var r readability.Readability

	input := strings.NewReader(`<html>
		<head>
			<title>hello world</title>
		</head>
		<body>
			<script src="/js/scripts.min.js" type="text/javascript"></script>
			<p>lorem ipsum</p>
			<script type="text/javascript" src="/js/scripts.min.js"></script>
			<script type="text/javascript">
			window.alert('Hello World');
			</script>
		</body>
		</html>`)

	if r, err = readability.New(input, "https://cixtor.com/blog"); err != nil {
		t.Fatalf("not initialized: %s", err)
	}

	if a, err = r.Parse(); err != nil {
		t.Fatalf("parser failure: %s", err)
	}

	// TODO(cixtor): modify test to validate the actual content.
	if a.TextContent != "" {
		t.Fatalf("scripts were not removed: %s", a.TextContent)
	}
}
