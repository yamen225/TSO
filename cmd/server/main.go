// @title           TSO Activation Service API
// @version         1.0
// @description     Microservice for TSO electrical asset activation using Hexagonal Architecture.
// @host            localhost:8080
// @BasePath        /
package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"

	"activation-service/internal/app/handler"
	"activation-service/internal/db"
	"activation-service/internal/domain/service"
	"activation-service/internal/infra/repo"

	_ "activation-service/docs"
)

func main() {
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	// Run database migrations before opening the connection pool
	if err := db.RunMigrations(connStr); err != nil {
		log.Fatalf("migrations failed: %v", err)
	}

	// Infrastructure layer
	assetRepo, err := repo.NewPostgresAssetRepositoryFromURL(connStr)
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	// Domain services
	greedyBaseline := service.NewGreedyBaseline(assetRepo)
	greedyDB       := service.NewGreedyDB(assetRepo)
	knapsackMemory := service.NewKnapsackMemory(assetRepo)
	knapsackDB     := service.NewKnapsackDB(assetRepo)

	// Application layer
	h := handler.NewActivationGinHandler(greedyBaseline, greedyDB, knapsackMemory, knapsackDB)

	r := gin.Default()

	// Swagger UI
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	v1 := r.Group("/api/v1/activation")
	v1.POST("/greedy-baseline", h.HandleGreedyBaseline)
	v1.POST("/greedy-db",       h.HandleGreedyDB)
	v1.POST("/knapsack-memory", h.HandleKnapsackMemory)
	v1.POST("/knapsack-db",     h.HandleKnapsackDB)

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
