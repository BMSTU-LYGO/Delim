package postgresx

import (
	"context"
	"fmt"
	"net/url"
	"strconv"

	"github.com/jackc/pgx/v5/pgxpool"
)

type Config struct {
	Host           string
	Port           int
	Database       string
	User           string
	Password       string
	SSLMode        string
	MaxConnections int32
}

func Open(ctx context.Context, cfg Config) (*pgxpool.Pool, error) {
	dsn := &url.URL{
		Scheme: "postgres",
		User:   url.UserPassword(cfg.User, cfg.Password),
		Host:   cfg.Host + ":" + strconv.Itoa(cfg.Port),
		Path:   cfg.Database,
	}
	query := dsn.Query()
	query.Set("sslmode", cfg.SSLMode)
	dsn.RawQuery = query.Encode()

	poolConfig, err := pgxpool.ParseConfig(dsn.String())
	if err != nil {
		return nil, fmt.Errorf("parse postgres config: %w", err)
	}
	if cfg.MaxConnections > 0 {
		poolConfig.MaxConns = cfg.MaxConnections
	}

	pool, err := pgxpool.NewWithConfig(ctx, poolConfig)
	if err != nil {
		return nil, fmt.Errorf("create postgres pool: %w", err)
	}
	if err := pool.Ping(ctx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}

	return pool, nil
}
