package brokerchaos

import (
	"context"
	"time"
)

// ProtocolType identifies the target broker protocol.
type ProtocolType string

const (
	ProtocolKafka    ProtocolType = "kafka"
	ProtocolRabbitMQ ProtocolType = "rabbitmq"
)

// MessageContext carries the metadata the evaluator needs to match a message against active chaos policies.
type MessageContext struct {
	Topic     string
	Protocol  ProtocolType
	Partition int32
	GetHeader func(key string) (string, bool)
}

// Evaluator decides what chaos (if any) to apply to a given message.
type Evaluator interface {
	Evaluate(ctx context.Context, msgCtx *MessageContext) (bool, time.Duration, error, string, string)
}
