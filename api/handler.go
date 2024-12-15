package api

import (
	"TestProject1/service"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pkg/errors"
)

type Handler struct {
	Service      *service.ClickService
	StatsService *service.StatsService
}

func (h *Handler) RegisterRoutes(router *gin.Engine) {
	router.GET("/counter/:bannerID", h.HandleIncrementClick)
	router.POST("/stats/:bannerID", h.HandleGetClickStats)
}

func (h *Handler) HandleIncrementClick(c *gin.Context) {
	bannerID, err := strconv.Atoi(c.Param("bannerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid banner ID"})
		return
	}

	timestamp := time.Now().Truncate(time.Minute)

	if err := h.Service.IncrementClick(bannerID, timestamp); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errors.Wrapf(err, "failed to increment click for banner %d at %v", bannerID, timestamp).Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Click incremented successfully"})
}

func (h *Handler) HandleGetClickStats(c *gin.Context) {
	bannerID, err := strconv.Atoi(c.Param("bannerID"))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid banner ID"})
		return
	}

	var request struct {
		TsFrom time.Time `json:"tsFrom"`
		TsTo   time.Time `json:"tsTo"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request body"})
		return
	}

	if request.TsFrom.After(request.TsTo) {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Start time must be before end time and not equal to end time"})
		return
	}

	stats, err := h.StatsService.GetClickStats(bannerID, request.TsFrom, request.TsTo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": errors.Wrapf(err, "failed to fetch click stats for banner %d between %v and %v", bannerID, request.TsFrom, request.TsTo).Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}
