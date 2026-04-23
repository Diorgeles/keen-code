package widgets

import (
	"testing"
)

func TestIsValidBaseURL_Empty(t *testing.T) {
	if err := isValidBaseURL(""); err != nil {
		t.Errorf("expected empty URL to be valid, got %v", err)
	}
}

func TestIsValidBaseURL_ValidHTTPS(t *testing.T) {
	cases := []string{
		"https://api.example.com",
		"https://api.example.com/v1",
		"http://localhost:8080",
		"http://localhost:8080/v1/",
	}
	for _, c := range cases {
		if err := isValidBaseURL(c); err != nil {
			t.Errorf("expected %q to be valid, got %v", c, err)
		}
	}
}

func TestIsValidBaseURL_InvalidScheme(t *testing.T) {
	cases := []string{
		"ftp://example.com",
		"example.com",
		"//example.com",
	}
	for _, c := range cases {
		if err := isValidBaseURL(c); err == nil {
			t.Errorf("expected %q to be invalid, got nil", c)
		}
	}
}

func TestIsValidBaseURL_MissingHost(t *testing.T) {
	if err := isValidBaseURL("https://"); err == nil {
		t.Error("expected URL with no host to be invalid")
	}
}
