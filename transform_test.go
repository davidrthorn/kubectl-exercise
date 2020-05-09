package main

import (
	"testing"
)

func TestGetDataKeyValuePairFormatsCorrectlyForGoodString(t *testing.T) {
	sut := HTTPDataPopulator{nil, ""}

	cases := []map[string]string{
		{
			"input": "someKey=someValue",
			"key":   "someKey",
			"value": "someValue",
		},
		{
			"input": "some-Key.with/otherchars=app.example.com",
			"key":   "some-Key.with/otherchars",
			"value": "app.example.com",
		},
	}

	for _, c := range cases {
		key, value, err := sut.getDataKeyValuePair(c["input"])
		if err != nil {
			t.Errorf("Got non-nil error for input '%s'", c["input"])
		}
		if key != c["key"] {
			t.Errorf("Key was incorrect. Got: %s; want %s", key, c["key"])
		}
		if value != c["value"] {
			t.Errorf("Value was incorrect. Got: %s; want %s", key, c["value"])
		}
	}
}

func TestGetDataKeyValuePairReturnsErrorForBadString(t *testing.T) {
	sut := HTTPDataPopulator{nil, ""}

	cases := []string{
		"someKey=",
		"someKey",
		"",
		"app.example.com",
	}

	for _, c := range cases {
		_, _, err := sut.getDataKeyValuePair(c)
		if err == nil {
			t.Errorf("Got nil error for input '%s'", c)
		}
	}
}

func TestValidURLReturnsURLForGoodInput(t *testing.T) {
	sut := HTTPDataPopulator{nil, ""}

	cases := [][]string{
		{"https://app.example.com", "https://app.example.com"}, // input, want
		{"http://app.example.com", "http://app.example.com"},
		{"app.example.com", "https://app.example.com"},
	}

	for _, c := range cases {
		want := c[1]
		got := sut.prefixURL(c[0])
		if got != want {
			t.Errorf("Incorrect URL returned. Expecting %s; got %s", want, got)
		}
	}
}

func TestValidURLIgnoresEmptyString(t *testing.T) {
	sut := HTTPDataPopulator{nil, ""}
	got := sut.prefixURL("")
	if got != "" {
		t.Errorf("Expected empty string. Got: %s", got)
	}
}

func TestTransformReturnsMapPopulatedWithDataForValidAnnotation(t *testing.T) {
}
