//
// Licensed to the Apache Software Foundation (ASF) under one
// or more contributor license agreements.  See the NOTICE file
// distributed with this work for additional information
// regarding copyright ownership.  The ASF licenses this file
// to you under the Apache License, Version 2.0 (the
// "License"); you may not use this file except in compliance
// with the License.  You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing,
// software distributed under the License is distributed on an
// "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY
// KIND, either express or implied.  See the License for the
// specific language governing permissions and limitations
// under the License.
//

package pulsar

import (
	"context"
	"fmt"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"

	log "github.com/apache/pulsar/pulsar-client-go/logutil"
	"github.com/stretchr/testify/assert"
)

func TestInvalidURL(t *testing.T) {
	client, err := NewClient(ClientOptions{})

	if client != nil || err == nil {
		t.Fatal("Should have failed to create client")
	}
}

func TestProducerConnectError(t *testing.T) {
	client, err := NewClient(ClientOptions{
		URL: "pulsar://invalid-hostname:6650",
	})

	assert.Nil(t, err)

	defer client.Close()

	producer, err := client.CreateProducer(ProducerOptions{
		Topic: "my-topic",
	})

	// Expect error in creating producer
	assert.Nil(t, producer)
	assert.NotNil(t, err)

	assert.Equal(t, err.(*Error).Result(), ConnectError)
}

func TestProducer(t *testing.T) {
	client, err := NewClient(ClientOptions{
		URL:                      "pulsar://localhost:6650",
		StatsIntervalInSeconds:   10,
		IOThreads:                1,
		OperationTimeoutSeconds:  30,
		ConcurrentLookupRequests: 1000,
		MessageListenerThreads:   5,
	})

	assert.Nil(t, err)
	defer client.Close()

	producer, err := client.CreateProducer(ProducerOptions{
		Topic:                   "my-topic",
		Name:                    "my-producer-name",
		SendTimeout:             10 * time.Second,
		Batching:                true,
		BatchingMaxMessages:     100,
		BatchingMaxPublishDelay: 10 * time.Millisecond,
		MaxPendingMessages:      100,
		BlockIfQueueFull:        true,
		CompressionType:         LZ4,
		Properties: map[string]string{
			"my-name": "test",
			"key":     "value",
		},
	})

	assert.Nil(t, err)
	defer producer.Close()

	assert.Equal(t, producer.Topic(), "persistent://public/default/my-topic")
	assert.Equal(t, producer.Name(), "my-producer-name")
	assert.Equal(t, producer.LastSequenceID(), int64(-1))

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		if err := producer.Send(ctx, ProducerMessage{
			Payload: []byte(fmt.Sprintf("hello-%d", i)),
		}); err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, producer.LastSequenceID(), int64(i))
	}
	assert.Equal(t, producer.LastSequenceID(), int64(9))
}

func TestProducerNoTopic(t *testing.T) {
	client, err := NewClient(ClientOptions{
		URL: "pulsar://localhost:6650",
	})

	if err != nil {
		t.Fatal(err)
		return
	}

	defer client.Close()

	producer, err := client.CreateProducer(ProducerOptions{
	})

	// Expect error in creating producer
	assert.Nil(t, producer)
	assert.NotNil(t, err)

	assert.Equal(t, err.(*Error).Result(), InvalidConfiguration)
}

func TestMessageRouter(t *testing.T) {
	// Create topic with 5 partitions
	httpPut("http://localhost:8080/admin/v2/persistent/public/default/my-partitioned-topic/partitions",
		5)

	client, err := NewClient(ClientOptions{
		URL: "pulsar://localhost:6650",
	})

	assert.Nil(t, err)
	defer client.Close()

	// Only subscribe on the specific partition
	consumer, err := client.Subscribe(ConsumerOptions{
		Topic:            "my-partitioned-topic-partition-2",
		SubscriptionName: "my-sub",
	})

	assert.Nil(t, err)
	defer consumer.Close()

	producer, err := client.CreateProducer(ProducerOptions{
		Topic: "my-partitioned-topic",
		MessageRouter: func(msg Message, tm TopicMetadata) int {
			fmt.Println("Routing message ", msg, " -- Partitions: ", tm.NumPartitions())
			return 2
		},
	})

	assert.Nil(t, err)
	defer producer.Close()

	ctx := context.Background()

	err = producer.Send(ctx, ProducerMessage{
		Payload:    []byte("hello"),
		SequenceID: 1234,
	})
	assert.Nil(t, err)
	assert.Equal(t, producer.LastSequenceID(), int64(1234))

	fmt.Println("PUBLISHED")

	// Verify message was published on partition 2
	msg, err := consumer.Receive(ctx)
	assert.Nil(t, err)
	assert.NotNil(t, msg)
	assert.Equal(t, string(msg.Payload()), "hello")
}

func TestProducerZstd(t *testing.T) {
	client, err := NewClient(ClientOptions{
		URL: "pulsar://localhost:6650",
	})

	assert.Nil(t, err)
	defer client.Close()

	producer, err := client.CreateProducer(ProducerOptions{
		Topic:           "my-topic",
		CompressionType: ZSTD,
	})

	assert.Nil(t, err)
	defer producer.Close()

	assert.Equal(t, producer.Topic(), "persistent://public/default/my-topic")
	assert.Equal(t, producer.Name(), "my-producer-name")

	ctx := context.Background()

	for i := 0; i < 10; i++ {
		if err := producer.Send(ctx, ProducerMessage{
			Payload: []byte(fmt.Sprintf("hello-%d", i)),
		}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestProducer_Flush(t *testing.T) {
	client, err := NewClient(ClientOptions{
		URL: "pulsar://localhost:6650",
	})
	assert.Nil(t, err)
	defer client.Close()

	topicName := "test-flush-in-producer"
	subName := "subscription-name"

	producer, err := client.CreateProducer(ProducerOptions{
		Topic: topicName,
		Properties: map[string]string{
			"producer-name": "test-producer-name",
			"producer-id":   "test-producer-id",
		},
	})
	assert.Nil(t, err)
	defer producer.Close()

	consumer, err := client.Subscribe(ConsumerOptions{
		Topic:            topicName,
		SubscriptionName: subName,
		Properties: map[string]string{
			"consumer-name": "test-consumer-name",
			"consumer-id":   "test-consumer-id",
		},
	})
	assert.Nil(t, err)
	defer consumer.Close()

	ctx := context.Background()
	for i := 0; i < 10; i++ {
		// Create a different message to send asynchronously
		asyncMsg := ProducerMessage{
			Payload: []byte(fmt.Sprintf("async-message-%d", i)),
		}
		// Attempt to send the message asynchronously and handle the response
		producer.SendAsync(ctx, asyncMsg, func(msg ProducerMessage, err error) {
			if err != nil {
				log.Fatal(err)
			}
			fmt.Printf("Message %s successfully published", msg.Payload)
		})
		producer.Flush()
	}
}
