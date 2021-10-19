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

func compareArticleContent(result *html.Node, expected *html.Node) error {
	// Make sure number of nodes is same
	resultNodesCount := len(children(result))
	expectedNodesCount := len(children(expected))
	if resultNodesCount != expectedNodesCount {
		return fmt.Errorf(
			"number of nodes is different, want %d got %d",
			expectedNodesCount,
			resultNodesCount,
		)
	}

	resultNode := result
	expectedNode := expected
	for resultNode != nil && expectedNode != nil {
		// Get node excerpt
		resultExcerpt := getNodeExcerpt(resultNode)
		expectedExcerpt := getNodeExcerpt(expectedNode)

		// Compare tag name
		resultTagName := tagName(resultNode)
		expectedTagName := tagName(expectedNode)
		if resultTagName != expectedTagName {
			return fmt.Errorf(
				"tag name is different\nwant: %s (%s)\ngot : %s (%s)",
				expectedTagName,
				expectedExcerpt,
				resultTagName,
				resultExcerpt,
			)
		}

		// Compare attributes
		resultAttrCount := len(resultNode.Attr)
		expectedAttrCount := len(expectedNode.Attr)
		if resultAttrCount != expectedAttrCount {
			return fmt.Errorf(
				"number of attributes is different\nwant: %d (%s)\ngot : %d (%s)",
				expectedAttrCount,
				expectedExcerpt,
				resultAttrCount,
				resultExcerpt,
			)
		}

		for _, resultAttr := range resultNode.Attr {
			expectedAttrVal := getAttribute(expectedNode, resultAttr.Key)
			switch resultAttr.Key {
			case "href", "src":
				resultAttr.Val = strings.TrimSuffix(resultAttr.Val, "/")
				expectedAttrVal = strings.TrimSuffix(expectedAttrVal, "/")
			}

			if resultAttr.Val != expectedAttrVal {
				return fmt.Errorf(
					"attribute %s is different\nwant: %s (%s)\ngot : %s (%s)",
					resultAttr.Key,
					expectedAttrVal,
					expectedExcerpt,
					resultAttr.Val,
					resultExcerpt,
				)
			}
		}

		// Compare text content
		resultText := strings.TrimSpace(textContent(resultNode))
		expectedText := strings.TrimSpace(textContent(expectedNode))

		resultText = strings.Join(strings.Fields(resultText), "\x20")
		expectedText = strings.Join(strings.Fields(expectedText), "\x20")

		if resultText != expectedText {
			return errColorDiff(
				"text content is different",
				expectedExcerpt,
				resultExcerpt,
			)
		}

		// Move to next node
		r := Readability{}
		resultNode = r.getNextNode(resultNode, false)
		expectedNode = r.getNextNode(expectedNode, false)
	}

	return nil
}
