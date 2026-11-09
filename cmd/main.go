package main

import (
	"log"

	"users/db"
	_ "users/cmd/docs" // сгенерированная swag init документация
	handle "users/internal/handlers"

	"github.com/gin-gonic/gin"
	swaggerFiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
)

// @title           Finance Tracker API
// @version         1.0
// @description     API учёта финансов: категории, транзакции и статистика (суммы хранятся в минорных единицах, например в копейках/центах).

// @contact.name   API Support
// @contact.email  abubakir72273313@gmail.com

// @license.name  MIT
// @license.url   https://opensource.org/licenses/MIT

// @host      localhost:8080
// @BasePath  /api
// @schemes   http
func main() {
	database := db.InitPostgres()
	h := handle.NewHandler(database)

	r := gin.Default()

	// Swagger UI: http://localhost:8080/swagger/index.html
	r.GET("/swagger/*any", ginSwagger.WrapHandler(swaggerFiles.Handler))

	h.RegisterRoutes(r.Group("/api"))

	log.Println("server started on :8080")
	if err := r.Run(":8080"); err != nil {
		log.Fatal("failed to start server: ", err)
	}
}
