package readability

import (
	"fmt"
	"strings"
	"testing"
)

func TestMaxElemsToParse(t *testing.T) {
	input := strings.NewReader(`<html>
		<head>
			<title>hello world</title>
		</head>
		<body>
			<p>lorem ipsum</p>
		</body>
		</html>`)

	parser := New()
	parser.MaxElemsToParse = 3
	_, err := parser.Parse(input, "https://cixtor.com/blog")

	if err.Error() != "too many elements: 5" {
		t.Fatalf("expecting failure due to MaxElemsToParse: %s", err)
	}
}

func TestRemoveScripts(t *testing.T) {
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

	a, err := New().Parse(input, "https://cixtor.com/blog")

	if err != nil {
		t.Fatalf("parser failure: %s", err)
	}

	if a.TextContent != "lorem ipsum" {
		t.Fatalf("scripts were not removed: %s", a.TextContent)
	}
}

func getNodeExcerpt(node *html.Node) string {
	outer := outerHTML(node)
	outer = strings.Join(strings.Fields(outer), "\x20")
	if len(outer) < 500 {
		return outer
	}
	return outer[:500]
}

func errColorDiff(label string, a string, b string) error {
	coloredA := ""
	coloredB := ""
	for i := 0; i < len(a); i++ {
		if b[i] == a[i] {
			coloredA += a[i : i+1]
			coloredB += b[i : i+1]
			continue
		}
		coloredA += "\x1b[0;92m" + a[i:] + "\x1b[0m"
		coloredB += "\x1b[0;91m" + b[i:] + "\x1b[0m"
		break
	}
	return fmt.Errorf("%s\n- %s\n+ %s", label, coloredA, coloredB)
}
