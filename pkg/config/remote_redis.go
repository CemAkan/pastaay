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

// WatchRedisPubSub ensures resilient, leak-free fleet synchronization.
func WatchRedisPubSub(ctx context.Context, client *redis.Client, channel string, wg *sync.WaitGroup, mgr *Manager) error {
	mgr.SetSensorStatus("redis", "initializing")

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			pubsub := client.Subscribe(ctx, channel)
			if _, err := pubsub.Receive(ctx); err != nil {
				mgr.SetSensorStatus("redis", "error")
				log.Printf("[Pastaay-Redis] subscribe failed on %q: %v", channel, err)
				if cerr := pubsub.Close(); cerr != nil {
					log.Printf("[Pastaay-Redis] pubsub close after subscribe failure: %v", cerr)
				}

				select {
				case <-ctx.Done():
					return
				case <-time.After(5 * time.Second):
				}
				continue
			}

			mgr.SetSensorStatus("redis", "connected")
			ackChannel := channel + ":ack"
			ch := pubsub.Channel()

		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case msg, ok := <-ch:
					if !ok {
						break Loop
					}
					var newCfg PastaayConfig
					if err := yaml.Unmarshal([]byte(msg.Payload), &newCfg); err != nil {
						log.Printf("[Pastaay-Redis] YAML parse failed: %v", err)
						sendAck(ctx, client, ackChannel, wg, "error", "YAML parse failed")
						continue
					}

					if err := newCfg.Validate(); err != nil {
						log.Printf("[Pastaay-Redis] payload rejected: %v", err)
						mgr.SetSensorStatus("redis", "rejected_payload")
						sendAck(ctx, client, ackChannel, wg, "rejected", err.Error())
						continue
					}

					mgr.Update(&newCfg)
					mgr.SetSensorStatus("redis", "healthy")
					sendAck(ctx, client, ackChannel, wg, "success", "Applied")
				}
			}

			if cerr := pubsub.Close(); cerr != nil {
				log.Printf("[Pastaay-Redis] pubsub close on loop exit: %v", cerr)
			}
			mgr.SetSensorStatus("redis", "disconnected")

			select {
			case <-ctx.Done():
				return
			case <-time.After(2 * time.Second):
			}
		}
	}()
	return nil
}

// sendAck dispatches asynchronous telemetry back to the control plane.
func sendAck(ctx context.Context, client *redis.Client, channel string, wg *sync.WaitGroup, status, message string) {
	if wg != nil {
		wg.Add(1)
	}
	go func() {
		if wg != nil {
			defer wg.Done()
		}
		// Detach cancellation but keep deadline
		ackCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 2*time.Second)
		defer cancel()
		ack := RedisAck{Status: status, Message: message}
		payload, err := json.Marshal(ack)
		if err != nil {
			log.Printf("[Pastaay-Redis] ack marshal failed: %v", err)
			return
		}
		if err := client.Publish(ackCtx, channel, payload).Err(); err != nil {
			log.Printf("[Pastaay-Redis] ack publish to %q failed: %v", channel, err)
		}
	}()
}
