package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

type ClientPaymentBlock struct {
	ID                uuid.UUID  `json:"id" gorm:"type:uuid;primaryKey"`
	ClientID          uuid.UUID  `json:"clientId" gorm:"type:uuid;index:idx_client_payment_blocks_client_id_active"`
	IsActive          bool       `json:"isActive" gorm:"column:is_active"`
	BlockType         string     `json:"blockType" gorm:"type:block_type_enum;column:block_type"`
	ReasonDescription string     `json:"reasonDescription" gorm:"column:reason_description"`
	CreatedAt         time.Time  `json:"createdAt" gorm:"column:created_at"`
	CreatedByUserID   string     `json:"createdByUserId" gorm:"column:created_by_user_id"`
	UnblockedAt       *time.Time `json:"unblockedAt" gorm:"column:unblocked_at"`
	UnblockedByUserID *string    `json:"unblockedByUserId" gorm:"column:unblocked_by_user_id"`
}

func (ClientPaymentBlock) TableName() string {
	return "client_payment_blocks"
}

type NewBlockRequest struct {
	BlockType         string `json:"blockType" binding:"required"`
	ReasonDescription string `json:"reasonDescription" binding:"required"`
	CreatedByUserID   string `json:"createdByUserId" binding:"required"`
}

type UnblockRequest struct {
	UnblockedByUserID string `json:"unblocked_by_user_id" binding:"required"`
}

var db *gorm.DB

func main() {
	dsn := "host=localhost user=postgres password=Googleapple123 dbname=postgres port=5432 sslmode=disable"
	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		panic(fmt.Sprintf("❌ Не удалось подключиться к БД: %v", err))
	}

	fmt.Println("✅ Подключение к БД успешно. Используем существующую таблицу client_payment_blocks.")

	router := gin.Default()

	router.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"*"},
		AllowMethods:     []string{"GET", "POST", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		ExposeHeaders:    []string{"Content-Length"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	}))

	v1 := router.Group("/internal/v1")
	{
		v1.POST("/clients/:clientId/payment-block", blockClient)
		v1.DELETE("/clients/:clientId/payment-block", unblockClient)
		v1.GET("/clients/:clientId/payment-block", getBlockStatus)
	}

	fmt.Println("🚀 API запущен на http://localhost:8080/internal/v1")
	router.Run(":8080")
}

func blockClient(c *gin.Context) {
	clientIdParam := c.Param("clientId")
	clientID, err := uuid.Parse(clientIdParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный UUID клиента"})
		return
	}

	var req NewBlockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var existing ClientPaymentBlock
	if err := db.Where("client_id = ? AND is_active = true", clientID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{"error": "Клиент уже заблокирован"})
		return
	}

	entry := ClientPaymentBlock{
		ID:                uuid.New(),
		ClientID:          clientID,
		IsActive:          true,
		BlockType:         req.BlockType,
		ReasonDescription: req.ReasonDescription,
		CreatedAt:         time.Now().UTC(),
		CreatedByUserID:   req.CreatedByUserID,
	}

	if err := db.Create(&entry).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка записи в БД: %v", err)})
		return
	}

	c.JSON(http.StatusCreated, entry)
}

func unblockClient(c *gin.Context) {
	clientIdParam := c.Param("clientId")
	clientID, err := uuid.Parse(clientIdParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный UUID клиента"})
		return
	}

	var req UnblockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var block ClientPaymentBlock
	if err := db.Where("client_id = ? AND is_active = true", clientID).First(&block).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Активная блокировка не найдена"})
		return
	}

	now := time.Now().UTC()
	block.IsActive = false
	block.UnblockedAt = &now
	block.UnblockedByUserID = &req.UnblockedByUserID

	if err := db.Save(&block).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Ошибка обновления БД: %v", err)})
		return
	}

	c.JSON(http.StatusOK, block)
}

func getBlockStatus(c *gin.Context) {
	clientIdParam := c.Param("clientId")
	clientID, err := uuid.Parse(clientIdParam)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Некорректный UUID клиента"})
		return
	}

	var block ClientPaymentBlock
	if err := db.Where("client_id = ? AND is_active = true", clientID).First(&block).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "Активных блокировок нет"})
		return
	}

	c.JSON(http.StatusOK, block)
}
