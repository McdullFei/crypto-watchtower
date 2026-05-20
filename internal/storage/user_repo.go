package storage

import "github.com/jackc/pgx/v5/pgxpool"

type UserRepo struct {
	DB *pgxpool.Pool
}
