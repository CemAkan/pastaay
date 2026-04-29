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
	// 1. Core: Smart Manager & Hot-Reload
	cfg, _ := config.LoadConfig("pastaay.yaml")
	cfgManager := config.NewManager(cfg)
	config.WatchConfig("pastaay.yaml", cfgManager.Update)

	// 2. Observability: Prometheus Metrics Server
	go metrics.StartServer(":2112")

	// 3. Redis: Chaos Network & Command Hooks
	rdb := redis.NewClient(&redis.Options{
		Addr:   getEnv("REDIS_ADDR", "localhost:6379"),
		Dialer: redischaos.NewChaosDialer(cfgManager),
	})
	rdb.AddHook(redischaos.NewChaosHook(cfgManager))

	// 4. SQL: Chaos Driver Integration
	sqlchaos.Register("pastaay-postgres", &pq.Driver{}, cfgManager)
	db, _ := sql.Open("pastaay-postgres", getEnv("DB_DSN", "postgres://pastaay:secret@localhost:5432/shortener?sslmode=disable"))

	// SMART MODE TEST: This table creation will NOT be sabotaged thanks to DefaultProtectedCommands!
	_, _ = db.Exec("CREATE TABLE IF NOT EXISTS pastaay_demo (id SERIAL PRIMARY KEY, note TEXT)")

	// 5. MongoDB (v2): Chaos Monitor & Dialer
	mongoOpts := options.Client().ApplyURI(getEnv("MONGO_URI", "mongodb://localhost:27017"))
	mongochaos.ApplyChaos(mongoOpts, cfgManager)
	mClient, _ := mongo.Connect(mongoOpts)
	defer mClient.Disconnect(context.Background())

	// 6. gRPC: Chaos Interceptors
	lis, _ := net.Listen("tcp", ":50051")
	s := grpc.NewServer(
		grpc.UnaryInterceptor(grpcchaos.UnaryInterceptor(cfgManager)),
		grpc.StreamInterceptor(grpcchaos.StreamInterceptor(cfgManager)),
	)
	reflection.Register(s)
	go s.Serve(lis)

	// 7. HTTP (Ritual): Chaos Middleware
	mux := http.NewServeMux()

	mux.HandleFunc("/api/v1/ping", func(w http.ResponseWriter, r *http.Request) {
		// Test Redis
		_ = rdb.Set(r.Context(), "last_ping", time.Now().String(), 0).Err()

		// Test SQL
		_, _ = db.Exec("INSERT INTO pastaay_demo (note) VALUES ('ping received')")

		// Test Mongo
		_ = mClient.Database("admin").RunCommand(r.Context(), bson.D{{Key: "ping", Value: 1}}).Err()

		json.NewEncoder(w).Encode(map[string]string{"status": "All systems operational (or sabotaged!)"})
	})

	mux.HandleFunc("/api/v1/shorten", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		// SQL Test
		_, err := db.Exec("INSERT INTO pastaay_demo (note) VALUES ('url shortened')")
		if err != nil {
			http.Error(w, "DB Error: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Redis Test
		err = rdb.Set(r.Context(), "short:xyz123", "https://golang.org", 10*time.Minute).Err()
		if err != nil && err != redis.Nil { // redis.Nil bizim chaos'tan gelebilir
			log.Printf("Redis Cache Error: %v", err)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"short_url": "http://localhost:8080/xyz123", "original_url": "https://golang.org"}`))
	})

	log.Println("Pastaay v1.5 [SMART MODE] FULL-STACK DEMO is live on :8080")
	log.Fatal(http.ListenAndServe(":8080", ritual.Middleware(cfgManager)(mux)))
}

func getEnv(k, f string) string {
	if v, ok := os.LookupEnv(k); ok {
		return v
	}
	return f
}
