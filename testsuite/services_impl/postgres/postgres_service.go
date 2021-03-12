package postgres

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
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

func (deployer PostgresService) IsAvailable() bool {
	return true
}
