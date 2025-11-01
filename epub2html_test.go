package main

import (
	"testing"
)

func TestNormalizeEpubPath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"path/to/file.html", "path/to/file.html"},
		{"path\\to\\file.html", "path/to/file.html"},
		{"path/../other/file.html", "other/file.html"},
		{"./path/to/file.html", "path/to/file.html"},
		{"", ""},
		{".", ""},
		{"path/./to/file.html", "path/to/file.html"},
	}

	for _, tt := range tests {
		result := normalizeEpubPath(tt.input)
		if result != tt.expected {
			t.Errorf("normalizeEpubPath(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestJoinEpubPath(t *testing.T) {
	tests := []struct {
		parts    []string
		expected string
	}{
		{[]string{"OEBPS", "text/chapter1.html"}, "OEBPS/text/chapter1.html"},
		{[]string{"OEBPS", "images", "test.jpg"}, "OEBPS/images/test.jpg"},
		{[]string{"", "path/file.html"}, "path/file.html"},
		{[]string{"path", ""}, "path"},
		{[]string{}, ""},
	}

	for _, tt := range tests {
		result := joinEpubPath(tt.parts...)
		if result != tt.expected {
			t.Errorf("joinEpubPath(%v) = %q, expected %q", tt.parts, result, tt.expected)
		}
	}
}

func TestEpubDir(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"OEBPS/text/chapter1.html", "OEBPS/text"},
		{"OEBPS/images/test.jpg", "OEBPS/images"},
		{"file.html", ""},
		{"", ""},
	}

	for _, tt := range tests {
		result := epubDir(tt.input)
		if result != tt.expected {
			t.Errorf("epubDir(%q) = %q, expected %q", tt.input, result, tt.expected)
		}
	}
}

func TestResolveEpubPath(t *testing.T) {
	tests := []struct {
		base     string
		rel      string
		expected string
	}{
		{"OEBPS/text", "../images/test.jpg", "OEBPS/images/test.jpg"},
		{"OEBPS/text", "styles/main.css", "OEBPS/text/styles/main.css"},
		{"OEBPS", "text/chapter1.html", "OEBPS/text/chapter1.html"},
		{"", "images/test.jpg", "images/test.jpg"},
		{"OEBPS/text/nested", "../../images/test.jpg", "OEBPS/images/test.jpg"},
	}

	for _, tt := range tests {
		result := resolveEpubPath(tt.base, tt.rel)
		if result != tt.expected {
			t.Errorf("resolveEpubPath(%q, %q) = %q, expected %q", tt.base, tt.rel, result, tt.expected)
		}
	}
}
