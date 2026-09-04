package http

import (
	"github.com/dungnt/dntproxy/internal/adapter/mcp"
	"github.com/dungnt/dntproxy/internal/port"
	"github.com/gin-gonic/gin"
)

// mountMCP registers the MCP analytics endpoint at /mcp behind the API key
// middleware, injecting the authenticated caller's identity so account
// selection and key policies apply to MCP-triggered analysis.
func mountMCP(r *gin.Engine, store port.CredentialStore, chatService port.ChatService) {
	identity := func(c *gin.Context) {
		id := &mcp.Identity{
			Policy:   extractAPIKeyPolicy(c),
			TenantID: GetTenantID(c),
		}
		if key := GetAPIKey(c); key != nil {
			id.APIKeyID = key.ID
		}
		c.Request = c.Request.WithContext(mcp.WithIdentity(c.Request.Context(), id))
		c.Next()
	}
	r.Any("/mcp", apiKeyMiddleware(store), identity, gin.WrapH(mcp.NewStreamableHandler(store, chatService)))
}
