package chainlink_oracle

import "fmt"

const (
	oracleEmail = "user@example.com"
	oraclePassword = "password"
	oracleWalletPassword = "walletPassword"
)


// ==========================================================================================
//								Helper methods
// ==========================================================================================

func getOracleEnvFile(chainId string, contractAddress string, gethClientIp string, gethClientWsPort string,
						postgresUsername string, postgresPassword string, postgresServer string, postgresPort string, postgresDatabase string) string {
	return fmt.Sprintf(`ROOT=/chainlink
LOG_LEVEL=debug
ETH_CHAIN_ID=%v
MIN_OUTGOING_CONFIRMATIONS=2
LINK_CONTRACT_ADDRESS=%v
CHAINLINK_TLS_PORT=0
SECURE_COOKIES=false
GAS_UPDATER_ENABLED=true
ALLOW_ORIGINS=*
ETH_URL=ws://%v:%v
DATABASE_URL=postgresql://%v:%v@%v:%v/%v`, chainId, contractAddress,
	gethClientIp, gethClientWsPort,
	postgresUsername, postgresPassword, postgresServer, postgresPort, postgresDatabase)
}

func getOracleApiFile(username string, password string) string {
	return fmt.Sprintf(`%v
%v`, username, password)
}

func getOraclePasswordFile(walletPassword string) string {
	return fmt.Sprintf("%v", walletPassword)
}