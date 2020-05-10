package main

import (
	"context"
	"errors"
	"testing"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
)

type mockTransformer struct {
	transform func(*v1.ConfigMap) (*v1.ConfigMap, error)
}

func (m mockTransformer) Transform(c *v1.ConfigMap) (*v1.ConfigMap, error) {
	return m.transform(c)
}

// There is a known issue with namespaces and events in fake client:
// https://github.com/kubernetes/kubernetes/pull/70343
// We have to mock the recorded ourselves
type mockRecorder struct {
	event func(object runtime.Object, eventtype string, reason string, message string)
}

func (r mockRecorder) Event(object runtime.Object, eventtype string, reason string, message string) {
	r.event(object, eventtype, reason, message)
}
func (r mockRecorder) Eventf(object runtime.Object, eventtype, reason, messageFmt string, args ...interface{}) {
}

func (r mockRecorder) PastEventf(object runtime.Object, timestamp metav1.Time, eventtype, reason, messageFmt string, args ...interface{}) {
}

func (r mockRecorder) AnnotatedEventf(object runtime.Object, annotations map[string]string, eventtype, reason, messageFmt string, args ...interface{}) {
}

func TestUpdateConfigMapUpdatesWatchedConfigMapWithTransformedVersion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := fake.NewSimpleClientset()

	configMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm"}}
	_, err := client.CoreV1().ConfigMaps("test-namespace").Create(configMap)
	if err != nil {
		t.Fatalf("Couldn't create configMap: %v", err)
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
		t.Fatalf("Couldn't retrieve configMap: %v", err)
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

func TestUpdateConfigMapRecordsErrorAsConfigMapEvent(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := fake.NewSimpleClientset()

	configMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm"}}
	_, err := client.CoreV1().ConfigMaps("test-namespace").Create(configMap)
	if err != nil {
		t.Fatalf("Couldn't create configMap: %v", err)
	}

	fakeReason := "test error reason"
	transformer := mockTransformer{
		transform: func(c *v1.ConfigMap) (*v1.ConfigMap, error) {
			return c, errors.New(fakeReason)
		},
	}

	factory := informers.NewSharedInformerFactory(client, 0)
	controller := NewController(client, factory, transformer)

	reasonCh := make(chan string)
	controller.recorder = mockRecorder{
		event: func(object runtime.Object, eventtype string, reason string, message string) {
			reasonCh <- reason
		},
	}

	go controller.Run(ctx)

	select {
	case reason := <-reasonCh:
		if reason != fakeReason {
			t.Fatalf("Got wrong reason for failure. Expected %s; got %s", fakeReason, reason)
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("ran out of time waiting for event reason")
	}
}
