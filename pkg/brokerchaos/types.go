package brokerchaos

import (
	"context"
	"time"
)

type ProtocolType string

const (
	ProtocolKafka    ProtocolType = "kafka"
	ProtocolRabbitMQ ProtocolType = "rabbitmq"
)

type MessageContext struct {
	Topic     string
	Protocol  ProtocolType
	Partition int32
	GetHeader func(key string) (string, bool)
}

type Evaluator interface {
	Evaluate(ctx context.Context, msgCtx *MessageContext) (bool, time.Duration, error, string, string)
}
