package rabbitmq

import (
	"encoding/json"
	"errors"
	"testing"
)

type fakeDelivery struct {
	body        []byte
	acked       bool
	nacked      bool
	nackRequeue bool
}

func (d *fakeDelivery) Body() []byte {
	return d.body
}

func (d *fakeDelivery) Ack(multiple bool) error {
	d.acked = true
	return nil
}

func (d *fakeDelivery) Nack(multiple, requeue bool) error {
	d.nacked = true
	d.nackRequeue = requeue
	return nil
}

func TestHandleConsumedDeliveryAcksSuccessfulMessages(t *testing.T) {
	body, err := json.Marshal(MessageMQParam{SessionID: "s1", Content: "hello", UserName: "alice", IsUser: true})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	delivery := &fakeDelivery{body: body}

	err = handleConsumedDelivery(delivery, func(param MessageMQParam) error {
		if param.SessionID != "s1" || param.UserName != "alice" {
			t.Fatalf("param = %#v", param)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("handleConsumedDelivery returned error: %v", err)
	}
	if !delivery.acked || delivery.nacked {
		t.Fatalf("acked=%v nacked=%v, want ack only", delivery.acked, delivery.nacked)
	}
}

func TestHandleConsumedDeliveryRequeuesRecoverableHandlerFailures(t *testing.T) {
	body, err := json.Marshal(MessageMQParam{SessionID: "s1", Content: "hello", UserName: "alice", IsUser: true})
	if err != nil {
		t.Fatalf("json.Marshal returned error: %v", err)
	}
	delivery := &fakeDelivery{body: body}

	err = handleConsumedDelivery(delivery, func(param MessageMQParam) error {
		return errors.New("db unavailable")
	})
	if err == nil {
		t.Fatal("handleConsumedDelivery returned nil for handler failure")
	}
	if !delivery.nacked || !delivery.nackRequeue || delivery.acked {
		t.Fatalf("acked=%v nacked=%v requeue=%v, want nack with requeue", delivery.acked, delivery.nacked, delivery.nackRequeue)
	}
}

func TestHandleConsumedDeliveryDropsMalformedJSON(t *testing.T) {
	delivery := &fakeDelivery{body: []byte("{bad json")}

	err := handleConsumedDelivery(delivery, func(param MessageMQParam) error {
		t.Fatal("handler should not be called for malformed JSON")
		return nil
	})
	if err == nil {
		t.Fatal("handleConsumedDelivery returned nil for malformed JSON")
	}
	if !delivery.nacked || delivery.nackRequeue || delivery.acked {
		t.Fatalf("acked=%v nacked=%v requeue=%v, want nack without requeue", delivery.acked, delivery.nacked, delivery.nackRequeue)
	}
}
