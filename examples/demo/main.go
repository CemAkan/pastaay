package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	"github.com/CemAkan/pastaay/pkg/config"
	"github.com/CemAkan/pastaay/pkg/grpcchaos"
	"github.com/CemAkan/pastaay/pkg/metrics"
	"github.com/CemAkan/pastaay/pkg/mongochaos"
	"github.com/CemAkan/pastaay/pkg/redischaos"
	"github.com/CemAkan/pastaay/pkg/ritual"
	"github.com/CemAkan/pastaay/pkg/sqlchaos"
)

func main() {
	log.Println("Pastaay Demo Application Starting...")

	// 1. Core: Smart Manager & Hot-Reload
	cfg, err := config.LoadConfig("pastaay.yaml")
	if err != nil {
		log.Fatalf("Fatal: Could not load pastaay.yaml: %v", err)
	}
	cfgManager := config.NewManager(cfg)
	config.WatchConfig("pastaay.yaml", cfgManager.Update)

	// 2. Observability: Prometheus Metrics Server
	go metrics.StartServer(":2112")

	// 3. Redis: Chaos Network & Command Hooks
	baseRedisDialer := &net.Dialer{Timeout: 5 * time.Second}
	rdb := redis.NewClient(&redis.Options{
		Addr:   getEnv("REDIS_ADDR", "localhost:6379"),
		Dialer: redischaos.NewChaosDialer(cfgManager, baseRedisDialer),
	})
	rdb.AddHook(redischaos.NewChaosHook(cfgManager))

	// Redis Retry for Docker Compose
	for i := 0; i < 10; i++ {
		if err := rdb.Ping(context.Background()).Err(); err == nil {
			log.Println("Redis Connected Successfully.")
			break
		}
		log.Println("Waiting for Redis...")
		time.Sleep(2 * time.Second)
	}

	// 4. SQL: Chaos Driver Integration
	sqlchaos.Register("pastaay-postgres", &pq.Driver{}, cfgManager)
	db, err := sql.Open("pastaay-postgres", getEnv("DB_DSN", "postgres://pastaay:secret@localhost:5432/shortener?sslmode=disable"))
	if err != nil {
		log.Fatalf("Fatal: Could not open SQL connection: %v", err)
	}

	// SQL Retry for Docker Compose
	for i := 0; i < 10; i++ {
		if err := db.Ping(); err == nil {
			log.Println("PostgreSQL Connected Successfully.")
			break
		}
		log.Println("Waiting for PostgreSQL...")
		time.Sleep(2 * time.Second)
	}

	// SQL SMART MODE TEST
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS pastaay_demo (id SERIAL PRIMARY KEY, note TEXT)")
	if err != nil {
		log.Printf("Warning: Failed to create table (Ignore if chaos is active globally): %v", err)
	}

	// 5. MongoDB (v2): Chaos Monitor & Dialer
	mongoOpts := options.Client().ApplyURI(getEnv("MONGO_URI", "mongodb://localhost:27017"))
	mongochaos.ApplyChaos(mongoOpts, cfgManager)
	mClient, err := mongo.Connect(mongoOpts)
	if err != nil {
		log.Fatalf("Fatal: Could not connect to MongoDB: %v", err)
	}
	defer mClient.Disconnect(context.Background())

	// Mongo retry for Docker Compose
	for i := 0; i < 10; i++ {
		if err := mClient.Ping(context.Background(), nil); err == nil {
			log.Println("MongoDB Connected Successfully.")
			break
		}
		log.Println("Waiting for MongoDB...")
		time.Sleep(2 * time.Second)
	}

	// MONGO SMART MODE TEST: 'createIndexes'
	_, err = mClient.Database("demo").Collection("logs").Indexes().CreateOne(context.Background(), mongo.IndexModel{
		Keys: bson.D{{Key: "timestamp", Value: 1}},
	})
	if err != nil {
		log.Printf("Warning: Failed to create Mongo Index: %v", err)
	}

	// 6. gRPC: Chaos Interceptors
	lis, err := net.Listen("tcp", ":50051")
	if err != nil {
		log.Fatalf("Fatal: Failed to listen on gRPC port: %v", err)
	}
	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpcchaos.UnaryInterceptor(cfgManager)),
		grpc.StreamInterceptor(grpcchaos.StreamInterceptor(cfgManager)),
	)
	reflection.Register(s)
	go s.Serve(lis)
	log.Println("gRPC Server listening on :50051")

	// 7. HTTP (Ritual): Chaos Middleware
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		_ = rdb.Set(r.Context(), "last_ping", time.Now().String(), 0).Err()
		_, _ = db.Exec("INSERT INTO pastaay_demo (note) VALUES ('ping received')")
		_ = mClient.Database("admin").RunCommand(r.Context(), bson.D{{Key: "ping", Value: 1}}).Err()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "Operational"})
	})

	mux.HandleFunc("/api/v1/shorten", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		_, err := db.Exec("INSERT INTO pastaay_demo (note) VALUES ('url shortened')")
		if err != nil {
			http.Error(w, "DB Error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		err = rdb.Set(r.Context(), "short:xyz123", "https://golang.org", 10*time.Minute).Err()
		if err != nil && err != redis.Nil {
			log.Printf("Redis Cache Error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"short_url": "http://localhost:8080/xyz123", "original_url": "https://golang.org"}`))
	})

	log.Println("Pastaay v1.5.1 [GOD MODE FINISHED] is live on :8080")
	log.Fatal(http.ListenAndServe(":8080", ritual.Middleware(cfgManager)(mux)))
}

func getEnv(k, f string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return f
}
