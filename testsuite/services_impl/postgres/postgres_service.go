package postgres

import (
	"database/sql"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
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
		return false
	}
	err = db.Ping()
	return err != nil
}
