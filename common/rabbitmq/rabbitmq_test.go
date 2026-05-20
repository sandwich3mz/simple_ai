package rabbitmq

import (
	"testing"

	"github.com/streadway/amqp"
)

func TestRabbitMQUsesDurableQueueAndPersistentMessages(t *testing.T) {
	if !defaultQueueDurable {
		t.Fatal("defaultQueueDurable = false, want true")
	}
	if defaultPublishingDeliveryMode != amqp.Persistent {
		t.Fatalf("defaultPublishingDeliveryMode = %d, want %d", defaultPublishingDeliveryMode, amqp.Persistent)
	}
}
