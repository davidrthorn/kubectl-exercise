package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// HTTPClient fetches data via HTTP
type HTTPClient interface {
	Get(url string) (*http.Response, error)
}

// HTTPDataPopulator updates a configMap's data based on a watched annotation
type HTTPDataPopulator struct {
	httpClient HTTPClient
	keyToWatch string
}

// Transform fetches data based on a watched annotation and populates the `data` field with it
func (p HTTPDataPopulator) Transform(configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	configMapCopy := configMap.DeepCopy()

	annotations := configMapCopy.GetAnnotations()
	dataStr, ok := annotations[p.keyToWatch]
	if !ok {
		return configMapCopy, nil // no watched annotation found
	}

	dataKey, URL, err := p.getDataKeyValuePair(dataStr)
	if err != nil {
		return configMapCopy, fmt.Errorf("could not decode annotation: %s", err)
	}

	fetchedValue, err := p.fetchSimpleBody(URL)
	if err != nil {
		return configMapCopy, fmt.Errorf("could not fetch data for annotation URL: %s", err)
	}

	if configMapCopy.Data == nil {
		configMapCopy.Data = map[string]string{}
	}
	configMapCopy.Data[dataKey] = fetchedValue

	return configMapCopy, nil
}

func (p HTTPDataPopulator) getDataKeyValuePair(watchValue string) (string, string, error) {
	spl := strings.Split(watchValue, "=")
	if len(spl) != 2 || spl[0] == "" || spl[1] == "" {
		return "", "", fmt.Errorf("watch values should be strings of the form 'key=value'. Value is '%s'", watchValue)
	}
	return spl[0], spl[1], nil
}

func (p HTTPDataPopulator) fetchSimpleBody(URL string) (string, error) {
	URL = p.prefixURL(URL)

	res, err := p.httpClient.Get(URL)
	if err != nil {
		return "", fmt.Errorf("request failed: %s", err)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("url responded with status: %d", res.StatusCode)
	}

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("could not read body: %s", err)
	}

	return strings.TrimSuffix(string(body), "\n"), nil
}

func (p HTTPDataPopulator) prefixURL(URL string) string {
	if URL != "" && !(strings.HasPrefix(URL, "http://") || strings.HasPrefix(URL, "https://")) {
		return "https://" + URL
	}
	return URL
}
