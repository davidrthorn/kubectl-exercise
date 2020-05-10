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

// Encountered a known issue with namespaces and events in fake client, so had to mock
// the event recorded manually. See github.com/kubernetes/kubernetes/pull/70343
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

func TestUpdateConfigMapUpdatesConfigMapWithTransformedVersion(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := fake.NewSimpleClientset()

	configMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm"}}
	_, err := client.CoreV1().ConfigMaps("test-namespace").Create(configMap)
	if err != nil {
		t.Fatalf("couldn't create configMap: %v", err)
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
		t.Fatalf("couldn't retrieve configMap: %v", err)
	}

	if updated.Data == nil {
		t.Fatalf("no Data found on configMap")
	}

	gotVal, ok := updated.Data["testKey"]
	if !ok {
		t.Fatalf("key not found in Data")
	}
	if gotVal != "testValue" {
		t.Fatalf("got wrong Data value. Expected %s; got %s", "testValue", gotVal)
	}
}

func TestUpdateConfigMapRecordsEventWithCorrectReasonOnFailureToTransform(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	client := fake.NewSimpleClientset()

	configMap := &v1.ConfigMap{ObjectMeta: metav1.ObjectMeta{Name: "test-cm"}}
	_, err := client.CoreV1().ConfigMaps("test-namespace").Create(configMap)
	if err != nil {
		t.Fatalf("couldn't create configMap: %v", err)
	}

	fakeReason := "test error reason"
	transformer := mockTransformer{
		transform: func(c *v1.ConfigMap) (*v1.ConfigMap, error) {
			return c, errors.New(fakeReason)
		},
	}

	factory := informers.NewSharedInformerFactory(client, 0)
	controller := NewController(client, factory, transformer)

	// Overwrite the controller's recorder with the mock
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
			t.Fatalf("got wrong reason for failure. Expected %s; got %s", fakeReason, reason)
		}
	case <-time.After(500 * time.Millisecond):
		t.Fatal("ran out of time waiting for event to be triggered")
	}
}
