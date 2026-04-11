package http

import (
	"encoding/json"
	"log"
	"strconv"
	"time"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/gin-gonic/gin"
)

func apiGetLogs(c *gin.Context) {
	logs, err := logger.Get().List(parseLogQuery(c))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, logs)
}

func apiGetLogSummary(c *gin.Context) {
	summary, err := logger.Get().Summary(parseLogQuery(c))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, summary)
}

func apiGetLogConnections(c *gin.Context) {
	connections, err := logger.Get().ConnectionSummaries(parseLogQuery(c))
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, connections)
}

func apiGetLogPrices(c *gin.Context) {
	prices, err := logger.Get().Prices()
	if err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	c.JSON(200, prices)
}

func apiLogStream(c *gin.Context) {
	c.Header("Content-Type", "text/event-stream")
	c.Header("Cache-Control", "no-cache")
	c.Header("Connection", "keep-alive")

	appLogger := logger.Get()
	ch := appLogger.Subscribe()
	defer appLogger.Unsubscribe(ch)

	sendLogs := func() bool {
		logs, err := appLogger.List(parseLogQuery(c))
		if err != nil {
			return false
		}
		data, _ := json.Marshal(logs)
		if _, err := c.Writer.Write([]byte("data: " + string(data) + "\n\n")); err != nil {
			return false
		}
		c.Writer.Flush()
		return true
	}

	if !sendLogs() {
		return
	}

	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	clientGone := c.Request.Context().Done()
	for {
		select {
		case <-clientGone:
			return
		case <-ch:
			if !sendLogs() {
				return
			}
		case <-ticker.C:
			c.Writer.Write([]byte(": keepalive\n\n"))
			c.Writer.Flush()
		}
	}
}

func apiClearLogs(c *gin.Context) {
	logger.Get().Clear()
	log.Printf("[LOG] Logs cleared by admin")
	c.JSON(200, gin.H{"ok": true})
}

func parseLogQuery(c *gin.Context) domain.LogQuery {
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "200"))
	return domain.LogQuery{
		ConnectionID: c.Query("connectionId"),
		Provider:     c.Query("provider"),
		Level:        c.Query("level"),
		Search:       c.Query("q"),
		Range:        c.DefaultQuery("range", "24h"),
		Limit:        limit,
	}
}

func apiCreatePrice(c *gin.Context) {
	var price domain.ModelPrice
	if err := c.ShouldBindJSON(&price); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	if err := logger.Get().InsertPrice(&price); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[LOG] Price created: %s/%s", price.Provider, price.ModelPattern)
	c.JSON(201, price)
}

func apiUpdatePrice(c *gin.Context) {
	id := c.Param("id")
	var price domain.ModelPrice
	if err := c.ShouldBindJSON(&price); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}
	price.ID = id
	if err := logger.Get().UpdatePrice(&price); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[LOG] Price updated: %s", id)
	c.JSON(200, price)
}

func apiDeletePrice(c *gin.Context) {
	id := c.Param("id")
	if err := logger.Get().DeletePrice(id); err != nil {
		c.JSON(500, gin.H{"error": err.Error()})
		return
	}
	log.Printf("[LOG] Price deleted: %s", id)
	c.JSON(200, gin.H{"ok": true})
}
