package http

import (
	"github.com/dungnt/dntproxy/internal/adapter/auth"
	"github.com/dungnt/dntproxy/internal/domain"
	"github.com/dungnt/dntproxy/internal/port"
)

func refreshOpenAIConnection(conn *domain.ProviderConnection, store port.CredentialStore) (*domain.ProviderConnection, error) {
	refreshSvc := auth.NewTokenRefreshService(store)
	updatedConn, err := refreshSvc.Refresh(conn)
	if err != nil {
		return nil, err
	}
	if err := store.UpdateConnection(updatedConn); err != nil {
		return nil, err
	}
	return updatedConn, nil
}
