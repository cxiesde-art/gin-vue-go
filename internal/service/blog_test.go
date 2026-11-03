package service

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	for input, expected := range map[string]string{
		"Hello, Vue Blog!": "hello-vue-blog",
		"  Go & Gin  ":     "go-gin",
		"中文文章":             "中文文章",
	} {
		if actual := Slugify(input); actual != expected {
			t.Errorf("Slugify(%q) = %q, want %q", input, actual, expected)
		}
	}
}

func TestPageCount(t *testing.T) {
	for _, values := range [][3]int64{{0, 10, 0}, {1, 10, 1}, {20, 10, 2}, {21, 10, 3}} {
		total, size, expected := values[0], values[1], values[2]
		if actual := PageCount(total, int(size)); int64(actual) != expected {
			t.Errorf("PageCount(%d, %d) = %d, want %d", total, size, actual, expected)
		}
	}
}

func TestNormalizeDisplayName(t *testing.T) {
	name, err := NormalizeDisplayName("  Vue Reader  ")
	if err != nil || name != "Vue Reader" {
		t.Fatalf("unexpected normalized display name: %q error=%v", name, err)
	}
	if _, err := NormalizeDisplayName(""); err != ErrInvalidDisplayName {
		t.Fatalf("empty display name error = %v, want %v", err, ErrInvalidDisplayName)
	}
	if _, err := NormalizeDisplayName(strings.Repeat("x", 121)); err != ErrInvalidDisplayName {
		t.Fatalf("long display name error = %v, want %v", err, ErrInvalidDisplayName)
	}
}
