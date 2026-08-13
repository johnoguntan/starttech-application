// MuchToDo API — StartTech deployment entry point
// All routes are mounted under /api so CloudFront can route /api/* to this service.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	redis "github.com/redis/go-redis/v9"
)

// ── Config ───────────────────────────────────────────────────────────

type Config struct {
	Port          string
	MongoURI      string
	DBName        string
	RedisHost     string
	RedisPassword string
	EnableCache   bool
	JWTSecretKey  string
	AllowedOrigins string
}

func loadConfig() Config {
	enableCache := os.Getenv("ENABLE_CACHE") == "true"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	dbName := os.Getenv("DB_NAME")
	if dbName == "" {
		dbName = "muchtodo"
	}
	allowedOrigins := os.Getenv("ALLOWED_ORIGINS")
	if allowedOrigins == "" {
		allowedOrigins = "*"
	}
	return Config{
		Port:           port,
		MongoURI:       os.Getenv("MONGO_URI"),
		DBName:         dbName,
		RedisHost:      os.Getenv("REDIS_HOST"),
		RedisPassword:  os.Getenv("REDIS_PASSWORD"),
		EnableCache:    enableCache,
		JWTSecretKey:   os.Getenv("JWT_SECRET_KEY"),
		AllowedOrigins: allowedOrigins,
	}
}

// ── Logger ────────────────────────────────────────────────────────────

func initLogger() {
	handler := slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	})
	slog.SetDefault(slog.New(handler))
}

// ── Redis Cache ───────────────────────────────────────────────────────

type Cache interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error
	Ping(ctx context.Context) error
}

type RedisCache struct{ client *redis.Client }
type NoOpCache struct{}

func newCache(cfg Config) Cache {
	if !cfg.EnableCache || cfg.RedisHost == "" {
		slog.Info("Caching disabled")
		return &NoOpCache{}
	}
	addr := fmt.Sprintf("%s:6379", cfg.RedisHost)
	rdb := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: cfg.RedisPassword,
		DB:       0,
	})
	if err := rdb.Ping(context.Background()).Err(); err != nil {
		slog.Error("Redis connection failed, falling back to no-op cache", "error", err)
		return &NoOpCache{}
	}
	slog.Info("Connected to Redis", "addr", addr)
	return &RedisCache{client: rdb}
}

func (r *RedisCache) Get(ctx context.Context, key string) (string, error) {
	return r.client.Get(ctx, key).Result()
}
func (r *RedisCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	b, _ := json.Marshal(value)
	return r.client.Set(ctx, key, b, ttl).Err()
}
func (r *RedisCache) Ping(ctx context.Context) error { return r.client.Ping(ctx).Err() }

func (n *NoOpCache) Get(ctx context.Context, key string) (string, error) {
	return "", redis.Nil
}
func (n *NoOpCache) Set(ctx context.Context, key string, value interface{}, ttl time.Duration) error {
	return nil
}
func (n *NoOpCache) Ping(ctx context.Context) error { return nil }

// ── MongoDB ───────────────────────────────────────────────────────────

func connectMongo(uri string) (*mongo.Client, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	client, err := mongo.Connect(ctx, options.Client().ApplyURI(uri))
	if err != nil {
		return nil, err
	}
	if err = client.Ping(ctx, nil); err != nil {
		return nil, err
	}
	return client, nil
}

// ── Handlers ──────────────────────────────────────────────────────────

type App struct {
	db    *mongo.Client
	cache Cache
	cfg   Config
}

func (a *App) healthCheck(c *gin.Context) {
	mongoStatus := "ok"
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := a.db.Ping(ctx, nil); err != nil {
		mongoStatus = fmt.Sprintf("error: %v", err)
	}

	cacheStatus := "disabled"
	if a.cfg.EnableCache {
		cacheStatus = "ok"
		if err := a.cache.Ping(context.Background()); err != nil {
			cacheStatus = fmt.Sprintf("error: %v", err)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"status":  "healthy",
		"service": "muchtodo-api",
		"checks": gin.H{
			"mongodb": mongoStatus,
			"redis":   cacheStatus,
		},
		"timestamp": time.Now().UTC().Format(time.RFC3339),
	})
}

func (a *App) getTodos(c *gin.Context) {
	collection := a.db.Database(a.cfg.DBName).Collection("todos")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cursor, err := collection.Find(ctx, bson.D{})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to fetch todos"})
		return
	}
	defer cursor.Close(ctx)

	var todos []bson.M
	if err = cursor.All(ctx, &todos); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to decode todos"})
		return
	}
	if todos == nil {
		todos = []bson.M{}
	}
	c.JSON(http.StatusOK, gin.H{"data": todos})
}

func (a *App) notFound(c *gin.Context) {
	c.JSON(http.StatusNotFound, gin.H{"error": "route not found"})
}

// ── Router ────────────────────────────────────────────────────────────

func setupRouter(app *App) *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// Structured JSON request logger
	r.Use(func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("request",
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"latency_ms", time.Since(start).Milliseconds(),
			"client_ip", c.ClientIP(),
		)
	})

	r.Use(gin.Recovery())

	// CORS middleware — allow CloudFront origin
	r.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", app.cfg.AllowedOrigins)
		c.Header("Access-Control-Allow-Credentials", "true")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,PATCH,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin,Content-Type,Authorization,Cookie")
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}
		c.Next()
	})

	// Root ping
	r.GET("/ping", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "pong"})
	})

	// All application routes live under /api so CloudFront's /api/* behavior
	// routes them here without any path stripping needed.
	// Health endpoints — full paths: /api/v1/health  /api/health
	const healthPath = "/api/v1/health"
	r.GET(healthPath, app.healthCheck)
	r.GET("/api/health", app.healthCheck)

	api := r.Group("/api")
	{
		// These register: /api/v1/health (already registered above)
		_ = api

		// Todo routes (representative subset — full app uses original source)
		v1 := api.Group("/v1")
		{
			v1.GET("/todos", app.getTodos)
		}
	}

	r.NoRoute(app.notFound)
	return r
}

// ── Server ────────────────────────────────────────────────────────────

func startServer(r *gin.Engine, port string) {
	srv := &http.Server{
		Addr:         ":" + port,
		Handler:      r,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		slog.Info("Server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("Server error", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	slog.Info("Shutting down server...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("Forced shutdown", "error", err)
		os.Exit(1)
	}
	slog.Info("Server exited")
}

// ── Main ──────────────────────────────────────────────────────────────

func main() {
	initLogger()

	cfg := loadConfig()
	slog.Info("Configuration loaded", "port", cfg.Port, "cache_enabled", cfg.EnableCache)

	if cfg.MongoURI == "" {
		slog.Error("MONGO_URI environment variable is required")
		os.Exit(1)
	}

	dbClient, err := connectMongo(cfg.MongoURI)
	if err != nil {
		slog.Error("MongoDB connection failed", "error", err)
		os.Exit(1)
	}
	defer func() {
		if err := dbClient.Disconnect(context.Background()); err != nil {
			slog.Error("MongoDB disconnect error", "error", err)
		}
	}()
	slog.Info("Connected to MongoDB")

	cache := newCache(cfg)
	app := &App{db: dbClient, cache: cache, cfg: cfg}

	router := setupRouter(app)
	startServer(router, cfg.Port)
}
