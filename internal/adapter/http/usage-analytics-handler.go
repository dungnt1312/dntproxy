package http

import (
	"log"
	"net/http"
	"strconv"

	"github.com/dungnt/dntproxy/internal/logger"
	"github.com/gin-gonic/gin"
)

var validPeriods = map[string]bool{"24h": true, "7d": true, "30d": true, "60d": true}

func apiGetUsageStats(c *gin.Context) {
	period := c.DefaultQuery("period", "7d")
	if !validPeriods[period] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid period. Use: 24h, 7d, 30d, 60d"})
		return
	}

	stats, err := logger.Get().UsageStats(c.Request.Context(), period)
	if err != nil {
		log.Printf("[USAGE] Stats error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch usage stats"})
		return
	}

	c.JSON(http.StatusOK, stats)
}

func apiGetUsageChart(c *gin.Context) {
	period := c.DefaultQuery("period", "7d")
	if !validPeriods[period] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid period. Use: 24h, 7d, 30d, 60d"})
		return
	}

	data, err := logger.Get().ChartData(c.Request.Context(), period)
	if err != nil {
		log.Printf("[USAGE] Chart error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch chart data"})
		return
	}

	c.JSON(http.StatusOK, data)
}

func apiGetRequestDetails(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	provider := c.Query("provider")
	startDate := c.Query("startDate")
	endDate := c.Query("endDate")

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	resp, err := logger.Get().RequestDetails(c.Request.Context(), page, pageSize, provider, startDate, endDate)
	if err != nil {
		log.Printf("[USAGE] Request details error: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to fetch request details"})
		return
	}

	c.JSON(http.StatusOK, resp)
}
