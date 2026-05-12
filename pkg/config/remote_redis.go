package config

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

type RedisAck struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// WatchRedisPubSub subscribes to a chaos channel and ensures distributed feedback via non-blocking ACKs.
// It accepts an optional WaitGroup to ensure all telemetry is dispatched before shutdown.
func WatchRedisPubSub(ctx context.Context, client *redis.Client, channel string, wg *sync.WaitGroup, reloadCallback func(*PastaayConfig)) error {
	pubsub := client.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}

	ackChannel := channel + ":ack"
	ch := pubsub.Channel()

	go func() {
		defer pubsub.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case msg, ok := <-ch:
				if !ok {
					return
				}

				var newCfg PastaayConfig
				if err := yaml.Unmarshal([]byte(msg.Payload), &newCfg); err != nil {
					sendAck(ctx, client, ackChannel, wg, "error", "YAML parse failed")
					continue
				}

				if err := newCfg.Validate(); err != nil {
					log.Printf("[Pastaay-Remote] Policy rejected: %v", err)
					sendAck(ctx, client, ackChannel, wg, "rejected", err.Error())
					continue
				}

				reloadCallback(&newCfg)
				log.Printf("[Pastaay-Remote] Engine memory hot-swapped via Redis")
				sendAck(ctx, client, ackChannel, wg, "success", "Policies applied")
			}
		}
	}()

	return nil
}

func sendAck(ctx context.Context, client *redis.Client, channel string, wg *sync.WaitGroup, status, message string) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}

		// Use background context for ACKs to ensure dispatch even if parent ctx is closing,
		// but with a very tight local timeout.
		ackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()

		ack := RedisAck{Status: status, Message: message}
		if payload, err := json.Marshal(ack); err == nil {
			client.Publish(ackCtx, channel, payload)
		}
	}()
}
