package main

import (
	"context"
	"fmt"
	"log"

	"github.com/jackc/pgx/v5/pgxpool"
)

var DB *pgxpool.Pool

func InitDB() {
	connStr := "postgresql://neondb_owner:npg_BJmu35adGLIV@ep-bold-field-adwk9hn6-pooler.c-2.us-east-1.aws.neon.tech/neondb?sslmode=require&channel_binding=require&pool_max_conns=5&disable_prepared_statements=true"

	var err error
	DB, err = pgxpool.New(context.Background(), connStr)
	if err != nil {
		log.Fatalf("❌ Gagal konek ke database: %v\n", err)
	}

	fmt.Println("✅ Koneksi ke Neon PostgreSQL berhasil!")
}
