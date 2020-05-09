package main

import (
	"testing"
)

func TestGetDataKeyValuePairFormatsValidStringsCorrectly(t *testing.T) {
	sut := DataPopulator{nil, ""}

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
			t.Errorf("Got non-nil error")
		}
		if key != c["key"] {
			t.Errorf("Key was incorrect. Got: %s; want %s", key, c["key"])
		}
		if value != c["value"] {
			t.Errorf("Value was incorrect. Got: %s; want %s", key, c["value"])
		}
	}
}
