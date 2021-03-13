package postgres

import (
	"database/sql"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/sirupsen/logrus"

	_ "github.com/lib/pq"
)

const (
	postgresDriverName = "postgres"
	postgresSuperUser = "postgres"
)

type PostgresService struct {
	serviceCtx *services.ServiceContext
}

func NewPostgresService(serviceCtx *services.ServiceContext) *PostgresService {
	return &PostgresService{serviceCtx: serviceCtx}
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (postgresService PostgresService) IsAvailable() bool {
	ipAddress := postgresService.serviceCtx.GetIPAddress()
	connStr := fmt.Sprintf("postgres://%v:%v@%v/%v?sslmode=disable", postgresSuperUser, postgresSuperUserPassword, ipAddress, databaseName)
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
