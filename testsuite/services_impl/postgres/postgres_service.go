package postgres

import (
	"database/sql"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/sirupsen/logrus"

	_ "github.com/lib/pq"
)

const (
	databaseName          = "postgres"
	port                  = 5432
	postgresDriverName    = "postgres"
	postgresSuperUsername = "postgres"

	postgresSuperUserPassword = "password"
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
		logrus.Infof("Got an error polling postgres: %v", err.Error())
		return false
	}
	err = db.Ping()
	if err != nil {
		logrus.Infof("Got an error pinging postgres: %v", err.Error())
	}
	return err != nil
}
