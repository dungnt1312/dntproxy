package http

import (
	"net/http"
	"strings"

	"github.com/dungnt/dntproxy/internal/service/autologin"
	"github.com/gin-gonic/gin"
)

// === OpenAI bulk auto-login ===
// Drives an automated browser to sign in a pasted list of accounts
// (email|password|2fa_secret) and saves each result as a connection.

func authOpenAIAutoLoginStart(svc *autologin.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Accounts     []string `json:"accounts"`     // one "email|password|2fa" per entry
			Text         string   `json:"text"`         // or a raw pasted block
			Workers      int      `json:"workers"`      //
			Headless     *bool    `json:"headless"`     // default: headed (captcha-safe)
			SkipExisting *bool    `json:"skipExisting"` // default: true — don't re-login healthy accounts
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		lines := req.Accounts
		if req.Text != "" {
			lines = append(lines, strings.Split(req.Text, "\n")...)
		}
		if len(lines) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "accounts is required"})
			return
		}

		headless := req.Headless != nil && *req.Headless
		skipExisting := req.SkipExisting == nil || *req.SkipExisting
		tenantID, keyID := authCallerIDs(c)
		status, err := svc.Start(lines, req.Workers, headless, skipExisting, autologin.Owner{TenantID: tenantID, KeyID: keyID})
		if err != nil {
			if strings.Contains(err.Error(), "already running") {
				c.JSON(http.StatusConflict, gin.H{"error": err.Error(), "status": status})
				return
			}
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"started":  true,
			"total":    status.Total,
			"workers":  status.Workers,
			"headless": status.Headless,
		})
	}
}

func authOpenAIAutoLoginStatus(svc *autologin.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, keyID := authCallerIDs(c)
		status, known := svc.Status(autologin.Owner{TenantID: tenantID, KeyID: keyID})
		if !known {
			if status.Total == 0 && !status.Running {
				// No job ever started for this caller.
				c.JSON(http.StatusOK, autologin.Status{Active: []string{}, Results: []autologin.AccountResult{}})
				return
			}
			c.JSON(http.StatusForbidden, gin.H{"error": "this auto-login run belongs to another dashboard key"})
			return
		}
		c.JSON(http.StatusOK, status)
	}
}

func authOpenAIAutoLoginStop(svc *autologin.Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		tenantID, keyID := authCallerIDs(c)
		status, ok := svc.Stop(autologin.Owner{TenantID: tenantID, KeyID: keyID})
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "no running auto-login job"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"stopped": true, "status": status})
	}
}
