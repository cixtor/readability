package readability_test

import (
	"strings"
	"testing"

	"github.com/cixtor/readability"
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

	reader, err := readability.New(input, "https://cixtor.com/blog/article")

	if err != nil {
		t.Fatalf("readability was not initialized: %s", err)
	}

	reader.MaxElemsToParse = 3

	if _, err := reader.Parse(); err == nil {
		t.Fatalf("MaxElemsToParse should fail if document is too long")
	}
}
