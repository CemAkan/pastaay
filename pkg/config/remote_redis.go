package config

import (
	"context"
	"encoding/json"
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
			_, err := pubsub.Receive(ctx)
			if err != nil {
				mgr.SetSensorStatus("redis", "error")
				pubsub.Close()
				
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
					break Loop // Graceful exit
				case msg, ok := <-ch:
					if !ok {
						break Loop
					}
					var newCfg PastaayConfig
					if err := yaml.Unmarshal([]byte(msg.Payload), &newCfg); err != nil {
						sendAck(ctx, client, ackChannel, wg, "error", "YAML parse failed")
						continue
					}

					if err := newCfg.Validate(); err != nil {
						mgr.SetSensorStatus("redis", "rejected_payload")
						sendAck(ctx, client, ackChannel, wg, "rejected", err.Error())
						continue
					}

					mgr.Update(&newCfg)
					mgr.SetSensorStatus("redis", "healthy")
					sendAck(ctx, client, ackChannel, wg, "success", "Applied")
				}
			}

			pubsub.Close()
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
		ackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ack := RedisAck{Status: status, Message: message}
		if payload, err := json.Marshal(ack); err == nil {
			client.Publish(ackCtx, channel, payload)
		}
	}()
}
