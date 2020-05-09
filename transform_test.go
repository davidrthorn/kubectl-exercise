package main

import (
	"bytes"
	"io/ioutil"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockHTTPClient struct {
	get func(URL string) (*http.Response, error)
}

func (m *MockHTTPClient) Get(URL string) (*http.Response, error) {
	return m.get(URL)
}

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
	mockClient := &MockHTTPClient{}
	sut := HTTPDataPopulator{mockClient, "watchThis"}
	returned := "some return string"
	reqURL := "https://app.example.com"

	mockClient.get = func(URL string) (*http.Response, error) {
		if URL != reqURL {
			t.Errorf("Mock http client received incorrect URL. Expected: %s; got: %s", reqURL, URL)
			t.FailNow()
		}
		res := &http.Response{
			Body:       ioutil.NopCloser(bytes.NewReader([]byte(returned))),
			StatusCode: http.StatusOK,
		}
		return res, nil
	}

	input := &corev1.ConfigMap{
		ObjectMeta: v1.ObjectMeta{
			Annotations: map[string]string{sut.keyToWatch: "someKey=" + reqURL},
		},
		Data: map[string]string{},
	}

	got, err := sut.Transform(input)
	gotVal, ok := got.Data["someKey"]
	if !ok {
		t.Errorf("didn't find relevant key in map. Error was: %v", err)
		t.FailNow()
	}
	if gotVal != returned {
		t.Errorf("configMap didn't correctly populate. Expected data value to be: %s; got: %s", returned, gotVal)
	}
}
