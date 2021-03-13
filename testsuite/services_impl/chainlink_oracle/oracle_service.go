package chainlink_oracle

import (
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"time"
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
	time.Sleep(300 * time.Second)
	return true
}

