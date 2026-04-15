package http

import (
	"net/http"

	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/dungnt/dntproxy/internal/service"
	"github.com/gin-gonic/gin"
)

// RegisterProfileRoutes registers profile management API routes.
func RegisterProfileRoutes(r *gin.RouterGroup, store port.CredentialStore) {
	svc := service.NewProfileService(store)

	r.GET("/profiles", apiListProfiles(svc))
	r.POST("/profiles", apiCreateProfile(svc))
	r.GET("/profiles/active", apiGetActiveProfile(svc, store))
	r.GET("/profiles/presets", apiListPresets())
	r.POST("/profiles/from-preset", apiCreateFromPreset(svc))
	r.POST("/profiles/deactivate", apiDeactivateProfile(svc))
	r.GET("/profiles/:name", apiGetProfile(svc))
	r.PUT("/profiles/:name", apiUpdateProfile(svc))
	r.DELETE("/profiles/:name", apiDeleteProfile(svc))
	r.POST("/profiles/:name/activate", apiActivateProfile(svc))
}

func apiListProfiles(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		profiles, activeProfile, err := svc.ListProfiles()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{
			"profiles":      profiles,
			"activeProfile": activeProfile,
		})
	}
}

func apiCreateProfile(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Name        string         `json:"name"`
			Description string         `json:"description"`
			Aliases     domain.AliasMap `json:"aliases"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}
		if req.Name == "" || len(req.Aliases) == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"error": "name and aliases are required"})
			return
		}

		profile, err := svc.CreateProfile(req.Name, req.Description, req.Aliases)
		if err != nil {
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, profile)
	}
}

func apiGetProfile(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		profile, err := svc.GetProfile(name)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, profile)
	}
}

func apiUpdateProfile(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		var req struct {
			AddAliases    domain.AliasMap `json:"addAliases"`
			RemoveAliases []string        `json:"removeAliases"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request"})
			return
		}

		if err := svc.UpdateProfileAliases(name, req.AddAliases, req.RemoveAliases); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func apiDeleteProfile(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if err := svc.DeleteProfile(name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func apiActivateProfile(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		name := c.Param("name")
		if err := svc.ActivateProfile(name); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true, "activated": name})
	}
}

func apiDeactivateProfile(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := svc.DeactivateProfile(); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}

func apiGetActiveProfile(svc *service.ProfileService, store port.CredentialStore) gin.HandlerFunc {
	return func(c *gin.Context) {
		cfg, err := store.Load()
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if cfg.Settings.ActiveProfile == "" {
			c.JSON(http.StatusOK, gin.H{"active": false})
			return
		}

		profile, err := svc.GetProfile(cfg.Settings.ActiveProfile)
		if err != nil {
			c.JSON(http.StatusOK, gin.H{
				"active":      true,
				"profileName": cfg.Settings.ActiveProfile,
				"error":       "profile data not found",
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"active":  true,
			"profile": profile,
		})
	}
}

func apiListPresets() gin.HandlerFunc {
	return func(c *gin.Context) {
		presets := make([]gin.H, 0)
		for _, name := range domain.ListPresetNames() {
			preset := domain.BuiltinPresets[name]
			presets = append(presets, gin.H{
				"name":        preset.Name,
				"description": preset.Description,
				"aliases":     preset.Aliases,
			})
		}
		c.JSON(http.StatusOK, presets)
	}
}

func apiCreateFromPreset(svc *service.ProfileService) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req struct {
			Preset string `json:"preset"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.Preset == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "preset name required"})
			return
		}

		profile, err := svc.CreateFromPreset(req.Preset)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusCreated, profile)
	}
}
