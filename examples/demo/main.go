package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/IBM/sarama"
	"github.com/lib/pq"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"github.com/CemAkan/pastaay/pkg/brokerchaos"
	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/mongochaos"
	"github.com/CemAkan/pastaay/pkg/redischaos"
	"github.com/CemAkan/pastaay/pkg/ritual"
	"github.com/CemAkan/pastaay/pkg/sqlchaos"
)

type brokerAdapter struct {
	mgr      *config.Manager
	protocol string
}

func (b *brokerAdapter) GetActivePolicies() []config.Policy {
	return b.mgr.GetActivePolicies(b.protocol)
}

func (b *brokerAdapter) IsCommandIgnored(protocol string, cmd string) bool {
	return b.mgr.IsCommandIgnored(protocol, cmd)
}

func main() {
	log.Println("[INFO] PASTAAY DEMO STARTING")

	cfgPath := "pastaay.yaml"
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("[FATAL] CONFIG LOAD ERROR: %v", err)
	}

	cfgManager := config.NewManager(cfg)
	config.WatchConfig(cfgPath, cfgManager.Update)

	go metrics.StartServer(":2112")

	//  Redis Setup
	rdb := redis.NewClient(&redis.Options{
		Addr:   getEnv("REDIS_ADDR", "redis:6379"),
		Dialer: redischaos.NewChaosDialer(cfgManager, nil),
	})
	rdb.AddHook(redischaos.NewChaosHook(cfgManager))

	//  SQL Setup
	sqlchaos.Register("pastaay-postgres", &pq.Driver{}, cfgManager)
	db, _ := sql.Open("pastaay-postgres", getEnv("DB_DSN", "postgres://pastaay:secret@db:5432/shortener?sslmode=disable"))

	//  Mongo Setup
	mOpts := options.Client().ApplyURI(getEnv("MONGO_URI", "mongodb://mongo:27017"))
	mongochaos.ApplyChaos(mOpts, cfgManager)
	mClient, _ := mongo.Connect(mOpts)

	kafkaEval := brokerchaos.NewEvaluator(&brokerAdapter{mgr: cfgManager, protocol: "kafka"})
	rabbitEval := brokerchaos.NewEvaluator(&brokerAdapter{mgr: cfgManager, protocol: "rabbitmq"})

	//  Kafka Setup
	go func() {
		kafkaAddr := getEnv("KAFKA_ADDR", "redpanda:9092")

		saramaCfg := sarama.NewConfig()
		saramaCfg.Producer.Return.Successes = true

		var kc sarama.Client
		var kcErr error
		for i := 0; i < 15; i++ {
			kc, kcErr = sarama.NewClient([]string{kafkaAddr}, saramaCfg)
			if kcErr == nil {
				break
			}
			log.Printf("[WAIT] Waiting for Kafka... (%d/15)", i+1)
			time.Sleep(2 * time.Second)
		}
		if kcErr != nil {
			log.Printf("[ERROR] Kafka unavailable, skipping: %v", kcErr)
			return
		}

		prod, err := sarama.NewSyncProducerFromClient(kc)
		if err != nil {
			log.Printf("[ERROR] Kafka producer error: %v", err)
			return
		}

		cons, err := sarama.NewConsumerFromClient(kc)
		if err != nil {
			log.Printf("[ERROR] Kafka consumer error: %v", err)
			return
		}

		pc, err := cons.ConsumePartition("events.stream", 0, sarama.OffsetNewest)
		if err != nil {
			log.Printf("[ERROR] Kafka partition error: %v", err)
			return
		}

		mid := brokerchaos.NewKafkaConsumerMiddleware(kafkaEval)

		// Dummy Producer
		go func() {
			for {
				_, _, err := prod.SendMessage(&sarama.ProducerMessage{Topic: "events.stream", Value: sarama.StringEncoder("payload")})
				if err != nil {
					log.Printf("[ERROR] Producer send failed: %v", err)
				}
				time.Sleep(2 * time.Second)
			}
		}()

		// Chaos Consumer
		for m := range pc.Messages() {
			drp, _ := mid.Intercept(context.Background(), m)
			if drp {
				log.Printf("[CHAOS] [KAFKA] Event Dropped")
			} else {
				log.Printf("[OK] [KAFKA] Processed Cleanly")
			}
		}
	}()

	//  RabbitMQ Setup
	go func() {
		rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
		var conn *amqp.Connection
		var amqpErr error
		for i := 0; i < 15; i++ {
			conn, amqpErr = amqp.Dial(rabbitURL)
			if amqpErr == nil {
				break
			}
			log.Printf("[WAIT] Waiting for RabbitMQ... (%d/15)", i+1)
			time.Sleep(2 * time.Second)
		}
		if amqpErr != nil {
			log.Printf("[ERROR] RabbitMQ unavailable, skipping: %v", amqpErr)
			return
		}

		ch, err := conn.Channel()
		if err != nil {
			log.Printf("[ERROR] RabbitMQ channel error: %v", err)
			return
		}

		q, err := ch.QueueDeclare("chaos.queue", false, false, false, false, nil)
		if err != nil {
			log.Printf("[ERROR] RabbitMQ queue error: %v", err)
			return
		}

		msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
		if err != nil {
			log.Printf("[ERROR] RabbitMQ consume error: %v", err)
			return
		}

		mid := brokerchaos.NewRabbitMQMiddleware(rabbitEval)

		// Dummy Publisher
		go func() {
			for {
				err := ch.PublishWithContext(context.Background(), "", q.Name, false, false, amqp.Publishing{Body: []byte("payload")})
				if err != nil {
					log.Printf("[ERROR] RabbitMQ publish failed: %v", err)
				}
				time.Sleep(2 * time.Second)
			}
		}()

		// Chaos Consumer
		for d := range msgs {
			drp, _ := mid.Intercept(context.Background(), &d)
			if drp {
				log.Printf("[CHAOS] [RABBITMQ] Event Dropped")
			} else {
				log.Printf("[OK] [RABBITMQ] Processed Cleanly")
			}
		}
	}()

	//  HTTP API
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		if err := mClient.Database("admin").RunCommand(r.Context(), bson.D{{Key: "ping", Value: 1}}).Err(); err != nil {
			log.Printf("[CHAOS] [MONGO] Connection/Command failed: %v", err)
		} else {
			log.Printf("[OK] [MONGO] Ping successful")
		}

		if err := rdb.Ping(r.Context()).Err(); err != nil {
			log.Printf("[CHAOS] [REDIS] Cache Miss or Error: %v", err)
		} else {
			log.Printf("[OK] [REDIS] Ping successful")
		}

		if err := db.PingContext(r.Context()); err != nil {
			log.Printf("[CHAOS] [SQL] Query failed: %v", err)
		} else {
			log.Printf("[OK] [SQL] Ping successful")
		}

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "PONG"})
	})

	//  Dummy HTTP Traffic Generator
	go func() {
		time.Sleep(5 * time.Second)
		client := &http.Client{Timeout: 2 * time.Second}
		for {
			resp, err := client.Get("http://localhost:8080/api/v1/ping")
			if err != nil {
				log.Printf("[CHAOS] [HTTP] API Request failed: %v", err)
			} else {
				log.Printf("[OK] [HTTP] API Status: %d", resp.StatusCode)
				resp.Body.Close()
			}
			time.Sleep(3 * time.Second)
		}
	}()

	log.Println("[INFO] Pastaay Integration Demo is LIVE on :8080")
	log.Fatal(http.ListenAndServe(":8080", ritual.Middleware(cfgManager)(mux)))
}

func getEnv(k, f string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return f
}
