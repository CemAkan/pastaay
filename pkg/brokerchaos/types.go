package brokerchaos

import (
	"context"
	"time"
)

// ProtocolType defines the target message broker system.
type ProtocolType string

const (
	ProtocolKafka    ProtocolType = "kafka"
	ProtocolRabbitMQ ProtocolType = "rabbitmq"
)

// MessageContext holds the non-payload metadata of an intercepted message.
type MessageContext struct {
	Topic     string
	Protocol  ProtocolType
	Partition int32
	Headers   map[string]string
}

// ChaosAction defines the engine's verdict on a specific message.
type ChaosAction string

const (
	ActionPass  ChaosAction = "pass"  // Let the message flow naturally
	ActionDrop  ChaosAction = "drop"  // Silently drop/ack the message without processing
	ActionDelay ChaosAction = "delay" // Hold the message processing for a duration
	ActionError ChaosAction = "error" // Force an unrecoverable broker error
)

// Evaluator is the core interface that decides the fate of a message based on active policies.
type Evaluator interface {
	Evaluate(ctx context.Context, msgCtx *MessageContext) (ChaosAction, time.Duration, error)
}
