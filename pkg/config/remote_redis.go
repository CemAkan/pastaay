package config

import (
	"context"
	"log"

	"github.com/redis/go-redis/v9"
	"gopkg.in/yaml.v3"
)

// WatchRedisPubSub subscribes to a Redis channel to dynamically overwrite
// the chaos engine's configuration in real-time across a distributed fleet.
func WatchRedisPubSub(ctx context.Context, client *redis.Client, channel string, reloadCallback func(*PastaayConfig)) error {
	pubsub := client.Subscribe(ctx, channel)

	// Wait for confirmation that the subscription is successfully created
	if _, err := pubsub.Receive(ctx); err != nil {
		return err
	}

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
				// yaml.Unmarshal natively supports both JSON and YAML payloads
				if err := yaml.Unmarshal([]byte(msg.Payload), &newCfg); err != nil {
					log.Printf("[Pastaay-Remote] Invalid payload received from Redis channel %q: %v", channel, err)
					continue
				}

				reloadCallback(&newCfg)
				log.Printf("[Pastaay-Remote] Engine memory hot-swapped via Redis PubSub channel: %s", channel)
			}
		}
	}()

	return nil
}
