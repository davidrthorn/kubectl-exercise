package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// DataPopulator updates a configMap's data based on a watched annotation
type DataPopulator struct {
	httpClient *http.Client
	keyToWatch string
}

// Transform fetches data based on a watched annotation and populates the `data` field with it
func (p *DataPopulator) Transform(configMap *corev1.ConfigMap) (*corev1.ConfigMap, error) {
	annotations := configMap.GetAnnotations()
	dataStr, ok := annotations[p.keyToWatch]
	if !ok {
		return nil, nil // no watched annotation found
	}

	dataKey, URL, err := p.getDataKeyValuePair(dataStr)
	if err != nil {
		return nil, fmt.Errorf("could not decode annotation: %s", err)
	}

	fetchedValue, err := p.fetchSimpleBody(URL)
	if err != nil {
		return nil, fmt.Errorf("could not fetch data for annotation URL: %s", err)
	}

	configMapCopy := configMap.DeepCopy()
	configMapCopy.Data[dataKey] = fetchedValue

	return configMapCopy, nil
}

func (p *DataPopulator) getDataKeyValuePair(watchValue string) (string, string, error) {
	spl := strings.Split(watchValue, "=")
	if len(spl) != 2 || spl[0] == "" || spl[1] == "" {
		return "", "", fmt.Errorf("watch values should be strings of the form 'key=value'. Value is '%s'", watchValue)
	}
	return spl[0], spl[1], nil
}

func (p *DataPopulator) fetchSimpleBody(URL string) (string, error) {
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

	return string(body), nil
}
