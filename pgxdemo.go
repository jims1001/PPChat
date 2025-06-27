package main

import (
	"context"
	"fmt"
	"github.com/jackc/pgx/v5/pgxpool"
	"os"
)

func main() {

	ctx := context.Background()
	databaseUrl := os.Getenv("DATABASE_URL")
	pool, err := pgxpool.New(ctx, databaseUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to database: %v\n", err)
		os.Exit(1)
	}

	defer pool.Close()

	var result string

	err = pool.QueryRow(ctx, "SELECT 'hello pgxpool'").Scan(&result)
	if err != nil {
		fmt.Fprintf(os.Stderr, "QueryRow failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(result, ":", result)

}
