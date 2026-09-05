package core_kafka

import (
	"context"
	"fmt"

	"github.com/segmentio/kafka-go"
)

type Producer struct {
	writer *kafka.Writer
}

// Message — сообщение для публикации. Key определяет партицию, поэтому
// события одного агрегата сохраняют относительный порядок.
type Message struct {
	Topic string
	Key   []byte
	Value []byte
}

func NewProducer(config Config) *Producer {
	writer := &kafka.Writer{
		Addr:                   kafka.TCP(config.Brokers...),
		Balancer:               &kafka.Hash{},
		RequiredAcks:           kafka.RequireAll,
		AllowAutoTopicCreation: true,
		WriteTimeout:           config.WriteTimeout,
	}

	return &Producer{
		writer: writer,
	}
}

func (p *Producer) Publish(ctx context.Context, messages ...Message) error {
	if len(messages) == 0 {
		return nil
	}

	kafkaMessages := make([]kafka.Message, len(messages))
	for i, message := range messages {
		kafkaMessages[i] = kafka.Message{
			Topic: message.Topic,
			Key:   message.Key,
			Value: message.Value,
		}
	}

	if err := p.writer.WriteMessages(ctx, kafkaMessages...); err != nil {
		return fmt.Errorf("write kafka messages: %w", err)
	}

	return nil
}

func (p *Producer) Close() error {
	if err := p.writer.Close(); err != nil {
		return fmt.Errorf("close kafka writer: %w", err)
	}

	return nil
}
