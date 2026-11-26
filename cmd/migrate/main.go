package main

import (
	"fmt"
	"net"
	"net/url"
	"os"
	"os/exec"

	"paygate/internal/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	if len(os.Args) < 2 {
		return fmt.Errorf("usage: go run ./cmd/migrate <up|down|version|force> [arguments]")
	}

	_, cfg, err := config.Init()
	if err != nil {
		return fmt.Errorf("load config.yml: %w", err)
	}
	pg := cfg.Database.Postgres
	databaseURL := postgresURL(pg.Username, pg.Password, pg.Host, pg.Port, pg.Name, pg.SSLMode)

	migratePath, err := exec.LookPath("migrate")
	if err != nil {
		return fmt.Errorf("golang-migrate CLI not found in PATH; install it before running migrations")
	}

	args := []string{"-path", "db/migrations", "-database", databaseURL}
	args = append(args, os.Args[1:]...)
	command := exec.Command(migratePath, args...)
	command.Stdout = os.Stdout
	command.Stderr = os.Stderr
	command.Stdin = os.Stdin
	if err := command.Run(); err != nil {
		return fmt.Errorf("migration command failed: %w", err)
	}
	return nil
}

func postgresURL(username, password, host string, port int, database, sslmode string) string {
	query := url.Values{}
	query.Set("sslmode", sslmode)
	return (&url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(username, password),
		Host:     net.JoinHostPort(host, fmt.Sprint(port)),
		Path:     "/" + database,
		RawQuery: query.Encode(),
	}).String()
}
