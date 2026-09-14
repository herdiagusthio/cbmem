package cbmem

import (
	"context"

	"github.com/herdiagusthio/cbmem/internal/storage"
)

// Client is the Go library for skill / callers to use directly.
type Client struct {
	store    *storage.Store
	repoName string
}

func Open(dbPath, repoName string) (*Client, error) {
	s, err := storage.NewStore(dbPath)
	if err != nil {
		return nil, err
	}
	if err := s.Migrate(); err != nil {
		s.Close()
		return nil, err
	}
	return &Client{store: s, repoName: repoName}, nil
}

func (c *Client) Close() error { return c.store.Close() }

func (c *Client) Search(ctx context.Context, q string, limit int) ([]*storage.Symbol, error) {
	return c.store.SearchSymbols(ctx, c.repoName, q, limit)
}

func (c *Client) Callers(ctx context.Context, qualified string, depth int) ([]*storage.Symbol, error) {
	return c.store.GetCallers(ctx, c.repoName, qualified, depth)
}

func (c *Client) Callees(ctx context.Context, qualified string, depth int) ([]*storage.Symbol, error) {
	return c.store.GetCallees(ctx, c.repoName, qualified, depth)
}

// Impact = transitive callers depth 3 (legacy; prefer GetImpact for routes/tests)
func (c *Client) Impact(ctx context.Context, qualified string) ([]*storage.Symbol, error) {
	return c.store.GetCallers(ctx, c.repoName, qualified, 3)
}

func (c *Client) GetImpact(ctx context.Context, qualified string, depth int) (*storage.ImpactResult, error) {
	return c.store.GetImpact(ctx, c.repoName, qualified, depth)
}
