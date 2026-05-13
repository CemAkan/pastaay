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

func WatchRedisPubSub(ctx context.Context, client *redis.Client, channel string, wg *sync.WaitGroup, mgr *Manager) error {
	pubsub := client.Subscribe(ctx, channel)
	if _, err := pubsub.Receive(ctx); err != nil {
		mgr.SetSensorStatus("redis", "error")
		return err
	}

	mgr.SetSensorStatus("redis", "connected")
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
					mgr.SetSensorStatus("redis", "rejected_payload")
					sendAck(ctx, client, ackChannel, wg, "rejected", err.Error())
					continue
				}

				mgr.Update(&newCfg)
				mgr.SetSensorStatus("redis", "healthy")
				sendAck(ctx, client, ackChannel, wg, "success", "Applied")
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
		ackCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		ack := RedisAck{Status: status, Message: message}
		if payload, err := json.Marshal(ack); err == nil {
			client.Publish(ackCtx, channel, payload)
		}
	}()
}
