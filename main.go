package main

import (
	"TestProject1/api"
	"TestProject1/config"
	"TestProject1/db"
	"TestProject1/service"
	"log"

	"github.com/gin-gonic/gin"
)

func main() {

	appConfig := config.NewConfig()

	database, err := db.NewDB(appConfig)
	if err != nil {
		log.Fatalf("Ошибка при подключении к базе данных: %v", err)
	}
	defer database.Close()

	redisClient := db.NewRedisClient()

	repo := &db.PSQLClickRepository{DB: database}
	clickService := service.NewClickService(repo, redisClient.Client)
	statsService := &service.StatsService{Repo: repo}

	handler := &api.Handler{
		Service:      clickService,
		StatsService: statsService,
	}

	r := gin.Default()
	handler.RegisterRoutes(r)

	if err := r.Run(":8080"); err != nil {
		log.Fatalf("Не удалось запустить сервер: %v", err)
	}
}
