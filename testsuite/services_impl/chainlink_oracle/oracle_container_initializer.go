package chainlink_oracle

import "fmt"


// ==========================================================================================
//								Helper methods
// ==========================================================================================

func getOracleEnvFile(chainId string, contractAddress string, gethClientIp string, gethClientWsPort string) string {
	return fmt.Sprintf(`ROOT=/chainlink
LOG_LEVEL=debug
ETH_CHAIN_ID=%v
MIN_OUTGOING_CONFIRMATIONS=2
LINK_CONTRACT_ADDRESS=%v
CHAINLINK_TLS_PORT=0
SECURE_COOKIES=false
GAS_UPDATER_ENABLED=true
ALLOW_ORIGINS=*
ETH_URL=ws://%v:%v`, chainId, contractAddress, gethClientIp, gethClientWsPort)
}
