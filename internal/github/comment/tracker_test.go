package comment

import (
	"context"
	"strings"
	"testing"
)

func TestFormatInitialBody(t *testing.T) {
	body := formatInitialBody()

	if !strings.Contains(body, "img src=") {
		t.Error("Initial body should contain spinner image")
	}

	if !strings.Contains(body, "Working on your request...") {
		t.Error("Initial body should contain 'Working on your request...' text")
	}
}

var _ = context.TODO
