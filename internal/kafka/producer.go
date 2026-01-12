package kafka

import (
    "context"
    "encoding/json"
    "time"

    "github.com/segmentio/kafka-go"
)

type Producer interface {
    Publish(ctx context.Context, topic string, msg interface{}) error
}

type producer struct {
    broker string
}

func NewProducer(broker string) Producer {
    return &producer{broker: broker}
}

func (p *producer) Publish(ctx context.Context, topic string, msg interface{}) error {
    b, err := json.Marshal(msg)
    if err != nil {
        return err
    }
    w := kafka.NewWriter(kafka.WriterConfig{
        Brokers: []string{p.broker},
        Topic:   topic,
        Balancer: &kafka.LeastBytes{},
    })
    defer w.Close()
    return w.WriteMessages(ctx, kafka.Message{
        Key:   nil,
        Value: b,
        Time:  time.Now(),
    })
}