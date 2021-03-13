package chainlink_oracle

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
)

type ChainlinkOracleService struct {
	serviceCtx *services.ServiceContext
}

func NewChainlinkOracleService(serviceCtx *services.ServiceContext) *ChainlinkOracleService {
	return &ChainlinkOracleService{serviceCtx: serviceCtx}
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (postgresService ChainlinkOracleService) IsAvailable() bool {
	return true
}

