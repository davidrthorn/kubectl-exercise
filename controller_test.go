package main

import (
	"context"
	"testing"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

type mockTransformer struct {
	transform func(*v1.ConfigMap) (*v1.ConfigMap, error)
}

func (m mockTransformer) Transform(c *v1.ConfigMap) (*v1.ConfigMap, error) {
	return m.transform(c)
}

func TestUpdateConfigMapUpdatesWatchedConfigMapWithTransformedVersion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := fake.NewSimpleClientset()

	configMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm"}}
	_, err := client.CoreV1().ConfigMaps("test-namespace").Create(configMap)
	if err != nil {
		t.Fatalf("Error injecting test configMap into mock client: %v", err)
	}

	transformer := mockTransformer{
		transform: func(c *v1.ConfigMap) (*v1.ConfigMap, error) {
			c = c.DeepCopy()
			c.Data = map[string]string{
				"testKey": "testValue",
			}
			return c, nil
		},
	}

	factory := informers.NewSharedInformerFactory(client, 0)
	controller := NewController(client, factory, transformer)

	controller.Run(ctx)

	updated, err := client.CoreV1().ConfigMaps("test-namespace").Get("test-cm", metav1.GetOptions{})
	if err != nil {
		t.Fatalf("Error retrieving configMap: %v", err)
	}

	if updated.Data == nil {
		t.Fatalf("No Data found on configMap")
	}

	gotVal, ok := updated.Data["testKey"]
	if !ok {
		t.Fatalf("Key not found in Data")
	}
	if gotVal != "testValue" {
		t.Fatalf("Got wrong Data value. Expected %s; got %s", "testValue", gotVal)
	}
}
