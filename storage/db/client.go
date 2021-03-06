package db

import (
	"flag"
	"fmt"
	"strings"
	"time"

	"github.com/google/logger"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
)

var (
	dbName     = flag.String("db_name", "omogenjudge", "Name of the database used for application storage")
	dbUser     = flag.String("db_user", "omogenjudge", "Name of the user that should connect to the database")
	dbPassword = flag.String("db_password", "omogenjudge", "Password used to connect to the database")
	dbHost     = flag.String("db_host", "localhost:5432", "Host in the form host:port that the database listens to")
)

var pool *sqlx.DB // Database connection pool.

func connString() string {
	hostPort := strings.Split(*dbHost, ":")
	return fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		hostPort[0], hostPort[1], *dbUser, *dbPassword, *dbName)
}

func initializePool() {
	pool = sqlx.MustConnect("postgres", connString())
	logger.Infof("Connected to database: %v", *dbName)
}

// Conn returns a new database connection for the judging database.
func Conn() *sqlx.DB {
	if pool == nil {
		initializePool()
	}
	return pool
}

// NewListener returns a new Postgres notify listener.
func NewListener() *pq.Listener {
	reportProblem := func(ev pq.ListenerEventType, err error) {
		if err != nil {
			logger.Fatalf("Postgres listener failure: %v", err)
		}
	}

	minReconn := 10 * time.Second
	maxReconn := time.Minute
	return pq.NewListener(connString(), minReconn, maxReconn, reportProblem)
}
