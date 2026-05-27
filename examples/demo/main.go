package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/joho/godotenv"

	"github.com/CemAkan/pastaay/pkg/brokerchaos"
	"github.com/CemAkan/pastaay/pkg/chaos"
	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/mongochaos"
	"github.com/CemAkan/pastaay/pkg/redischaos"
	"github.com/CemAkan/pastaay/pkg/ritual"
	"github.com/CemAkan/pastaay/pkg/sqlchaos"
	"github.com/CemAkan/pastaay/pkg/tracing"
	"github.com/CemAkan/pastaay/web"
	"github.com/IBM/sarama"
	"github.com/lib/pq"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
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
	_ = godotenv.Load()
	log.Println("[INFO] PASTAAY ENGINE BOOTING...")

	mainCtx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var wg sync.WaitGroup

	// Trace Provider Lifecycle
	shutdownOTel, err := tracing.InitProvider(mainCtx, getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""))
	if err != nil {
		log.Printf("[WARN] OpenTelemetry initialization failed. Tracing disabled: %v", err)
		shutdownOTel = func(context.Context) error { return nil }
	}
	defer func() {
		log.Println("[INFO] Finalizing OpenTelemetry: Flushing spans...")
		flushCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownOTel(flushCtx); err != nil {
			log.Printf("[WARN] OTel shutdown: %v", err)
		}
	}()

	// Configuration & Hot-Reload Watcher
	cfgPath := getEnv("CONFIG_PATH", "pastaay.yaml")
	cfg, err := config.LoadConfig(cfgPath)
	if err != nil {
		log.Fatalf("[FATAL] Core configuration load failure: %v", err)
	}
	cfgManager := config.NewManager(cfg)
	if err := config.WatchConfig(cfgPath, cfgManager.Update); err != nil {
		log.Printf("[WARN] config hot-reload watcher disabled: %v", err)
	}

	// Admin Server
	adminMux := http.NewServeMux()
	adminMux.Handle("/metrics", promhttp.Handler())

	webhookToken := getEnv("PASTAAY_WEBHOOK_TOKEN", "")
	if webhookToken == "" {
		log.Println("[WARN] PASTAAY_WEBHOOK_TOKEN is empty. Webhook endpoint is unprotected!")
	}

	adminMux.HandleFunc("/chaos/webhook", config.WebhookHandler(webhookToken, cfgManager.Update))
	adminMux.HandleFunc("/chaos/export", config.ExportHandler(cfgManager))

	web.RegisterHandlers(adminMux, cfgManager)

	adminAddr := getEnv("ADMIN_ADDR", ":2112")
	adminSrv := &http.Server{
		Addr:              adminAddr,
		Handler:           adminMux,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      15 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	go func() {
		log.Printf("[INFO] Pastaay Admin Server listening on %s", adminAddr)
		if err := adminSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Printf("[ERROR] Admin server crashed: %v", err)
			stop()
		}
	}()

	// Infrastructure Datastores
	rdb, db, mClient := initDatastores(cfgManager)
	if rdb != nil {
		defer rdb.Close()
	}
	if db != nil {
		defer db.Close()
	}
	if mClient != nil {
		defer func() {
			disCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()
			if err := mClient.Disconnect(disCtx); err != nil {
				log.Printf("[WARN] Mongo disconnect: %v", err)
			}
		}()
	}

	// Remote Control Sensors
	redisChannel := getEnv("REDIS_CONFIG_CHANNEL", "pastaay:chaos:policies")
	if err := config.WatchRedisPubSub(mainCtx, rdb, redisChannel, &wg, cfgManager); err != nil {
		log.Printf("[WARN] Redis PubSub sensor failed: %v", err)
	}

	// Prometheus Health Sync (10s)
	go func() {
		ticker := time.NewTicker(10 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-mainCtx.Done():
				return
			case <-ticker.C:
				statuses := cfgManager.GetSensorStatuses()
				for sensor, status := range statuses {
					val := 0.0
					if status == "healthy" || status == "connected" || status == "initializing" {
						val = 1.0
					}
					metrics.SensorStatus.WithLabelValues(sensor).Set(val)
				}
			}
		}
	}()

	// Broker & Background Lifecycles
	wg.Add(4)
	go initKafka(mainCtx, cfgManager, &wg)
	go initRabbitMQ(mainCtx, cfgManager, &wg)
	appAddr := getEnv("APP_ADDR", ":8080")
	appHealthURL := getEnv("APP_HEALTH_URL", "http://localhost"+appAddr+"/api/v1/ping")
	go startBackgroundPinger(mainCtx, &wg, appHealthURL)
	go func() {
		defer wg.Done()
		chaos.MonitorAndTrigger(mainCtx, cfgManager)
	}()

	// Hardened Server Setup
	mux := setupRouter(cfgManager, rdb, db, mClient)
	chaosHandler := ritual.Middleware(cfgManager)(mux)

	srv := &http.Server{
		Addr:              appAddr,
		Handler:           chaosHandler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      10 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	// Graceful Shutdown Orchestration
	done := make(chan struct{})
	go func() {
		<-mainCtx.Done()
		log.Println("[WARN] Shutdown signal captured. Draining resources...")

		shutdownCtx, shutCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutCancel()

		if err := srv.Shutdown(shutdownCtx); err != nil {
			log.Printf("[WARN] HTTP server shutdown: %v", err)
		}
		if err := adminSrv.Shutdown(shutdownCtx); err != nil {
			log.Printf("[WARN] Admin server shutdown: %v", err)
		}

		wg.Wait()
		close(done)
	}()

	log.Printf("[INFO] Pastaay Demo is LIVE on %s", appAddr)
	if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Printf("[ERROR] HTTP server crashed: %v", err)
		stop()
	}

	<-done
	log.Println("[INFO] Pastaay successfully decommissioned.")
}

func initDatastores(mgr *config.Manager) (*redis.Client, *sql.DB, *mongo.Client) {
	rdb := redis.NewClient(&redis.Options{
		Addr:   getEnv("REDIS_ADDR", "redis:6379"),
		Dialer: redischaos.NewChaosDialer(mgr, nil),
	})
	rdb.AddHook(redischaos.NewChaosHook(mgr))

	sqlchaos.Register("pastaay-postgres", &pq.Driver{}, mgr)
	db, dbErr := sql.Open("pastaay-postgres", getEnv("DB_DSN", "postgres://pastaay:secret@db:5432/shortener?sslmode=disable"))
	if dbErr != nil {
		log.Printf("[ERROR] Postgres init failure (DSN parse): %v", dbErr)
		db = nil
	}

	mOpts := options.Client().ApplyURI(getEnv("MONGO_URI", "mongodb://mongo:27017"))
	mongochaos.ApplyChaos(mOpts, mgr)
	mClient, err := mongo.Connect(mOpts)
	if err != nil {
		log.Printf("[ERROR] MongoDB failure: %v", err)
	}

	return rdb, db, mClient
}

func initKafka(ctx context.Context, mgr *config.Manager, wg *sync.WaitGroup) {
	defer wg.Done()

	kafkaAddr := getEnv("KAFKA_ADDR", "redpanda:9092")
	saramaCfg := sarama.NewConfig()
	saramaCfg.Producer.Return.Successes = true

	var kc sarama.Client
	var err error
	for i := 0; i < 10; i++ {
		kc, err = sarama.NewClient([]string{kafkaAddr}, saramaCfg)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		log.Printf("[ERROR] Kafka integration disabled. Connection failed: %v", err)
		return
	}
	defer kc.Close()

	prod, err := sarama.NewSyncProducerFromClient(kc)
	if err != nil {
		log.Printf("[ERROR] Kafka producer init failed: %v", err)
		return
	}
	defer prod.Close()

	childCtx, childCancel := context.WithCancel(ctx)
	defer childCancel()

	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-childCtx.Done():
				return
			case <-ticker.C:
				_, _, _ = prod.SendMessage(&sarama.ProducerMessage{
					Topic: "events.stream",
					Value: sarama.StringEncoder("payload"),
				})
			}
		}
	}()

	cons, err := sarama.NewConsumerFromClient(kc)
	if err != nil {
		return
	}
	defer cons.Close()

	pc, err := cons.ConsumePartition("events.stream", 0, sarama.OffsetNewest)
	if err != nil {
		return
	}
	defer pc.Close()

	evaluator := brokerchaos.NewEvaluator(&brokerAdapter{mgr: mgr, protocol: "kafka"})
	middleware := brokerchaos.NewKafkaConsumerMiddleware(evaluator)

	for {
		select {
		case <-ctx.Done():
			childCancel()
			workerWg.Wait()
			return
		case m, ok := <-pc.Messages():
			if !ok {
				childCancel()
				workerWg.Wait()
				return
			}
			drp, _ := middleware.Intercept(ctx, m)
			if drp {
				log.Printf("[CHAOS] [KAFKA] Message dropped")
			}
		}
	}
}

func initRabbitMQ(ctx context.Context, mgr *config.Manager, wg *sync.WaitGroup) {
	defer wg.Done()

	rabbitURL := getEnv("RABBITMQ_URL", "amqp://guest:guest@rabbitmq:5672/")
	var conn *amqp.Connection
	var err error
	for i := 0; i < 10; i++ {
		conn, err = amqp.Dial(rabbitURL)
		if err == nil {
			break
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(2 * time.Second):
		}
	}
	if err != nil {
		log.Printf("[ERROR] RabbitMQ integration disabled. Connection failed: %v", err)
		return
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return
	}
	defer ch.Close()

	q, _ := ch.QueueDeclare("chaos.queue", false, false, false, false, nil)
	msgs, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		return
	}

	evaluator := brokerchaos.NewEvaluator(&brokerAdapter{mgr: mgr, protocol: "rabbitmq"})
	middleware := brokerchaos.NewRabbitMQMiddleware(evaluator)

	childCtx, childCancel := context.WithCancel(ctx)
	defer childCancel()

	var workerWg sync.WaitGroup
	workerWg.Add(1)
	go func() {
		defer workerWg.Done()
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-childCtx.Done():
				return
			case <-ticker.C:
				if err := ch.PublishWithContext(ctx, "", q.Name, false, false,
					amqp.Publishing{Body: []byte("payload")}); err != nil && ctx.Err() == nil {
					log.Printf("[WARN] rabbitmq publish failed: %v", err)
				}
			}
		}
	}()

	for {
		select {
		case <-ctx.Done():
			childCancel()
			workerWg.Wait()
			return
		case d, ok := <-msgs:
			if !ok {
				childCancel()
				workerWg.Wait()
				return
			}
			drp, _ := middleware.Intercept(ctx, &d)
			if drp {
				log.Printf("[CHAOS] [RABBITMQ] Message dropped")
			}
		}
	}
}

func setupRouter(mgr *config.Manager, rdb *redis.Client, db *sql.DB, mClient *mongo.Client) *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		if mClient != nil {
			_ = mClient.Database("admin").RunCommand(r.Context(), bson.D{{Key: "ping", Value: 1}})
		}
		if rdb != nil {
			_ = rdb.Ping(r.Context())
		}
		if db != nil {
			_ = db.PingContext(r.Context())
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "message": "PONG"})
	})
	return mux
}

func startBackgroundPinger(ctx context.Context, wg *sync.WaitGroup, healthURL string) {
	defer wg.Done()

	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	client := &http.Client{Timeout: 2 * time.Second}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			req, err := http.NewRequestWithContext(ctx, "GET", healthURL, nil)
			if err != nil {
				continue
			}
			if resp, err := client.Do(req); err == nil {
				_, _ = io.Copy(io.Discard, resp.Body)
				_ = resp.Body.Close()
			}
		}
	}
}

func getEnv(k, f string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return f
}
