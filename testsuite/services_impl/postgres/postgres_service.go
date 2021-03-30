package postgres

import (
	"database/sql"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"time"

	_ "github.com/lib/pq"
)

const (
	databaseName          = "postgres"
	port                  = 5432
	postgresDriverName    = "postgres"
	postgresSuperUsername = "postgres"

	postgresSuperUserPassword = "password"

	numSuccessfulPingsForAvailability = 3
	timeBetweenPings = 500 * time.Millisecond
)

type PostgresService struct {
	serviceCtx *services.ServiceContext
}

func NewPostgresService(serviceCtx *services.ServiceContext) *PostgresService {
	return &PostgresService{serviceCtx: serviceCtx}
}

func (postgresService PostgresService) GetSuperUsername() string {
	return postgresSuperUsername
}

func (postgresService PostgresService) GetSuperUserPassword() string {
	return postgresSuperUserPassword
}

func (postgresService PostgresService) GetDatabaseName() string {
	return databaseName
}

func (postgresService PostgresService) GetPort() int {
	return port
}

func (postgresService PostgresService) GetIPAddress() string {
	return postgresService.serviceCtx.GetIPAddress()
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (postgresService PostgresService) IsAvailable() bool {
	ipAddress := postgresService.serviceCtx.GetIPAddress()
	connStr := fmt.Sprintf("postgres://%v:%v@%v/%v?sslmode=disable", postgresSuperUsername, postgresSuperUserPassword, ipAddress, databaseName)
	db, err := sql.Open(postgresDriverName, connStr)
	if err != nil {
		return false
	}
	// Need multiple pings to be successful, since the database seems to come up, shut down, and then come back up
	for i := 0; i < numSuccessfulPingsForAvailability; i++ {
		if err := db.Ping(); err != nil {
			return false
		}
		// Minor optimization: skip the last sleep if we're on the last iteration
		if i < numSuccessfulPingsForAvailability - 1 {
			time.Sleep(timeBetweenPings)
		}
	}
	return true
}
