package execution_impl

type ChainlinkTestsuiteArgs struct {
	GethServiceImage	string 			`json:"gethServiceImage"`
	ChainlinkContractDeployerImage	string	`json:"chainlinkContractDeployerImage"`
	ChainlinkOracleImage	string	`json:"chainlinkOracleImage"`
	PostgresImage	string	`json:"postgresImage"`

	// Indicates that this testsuite is being run as part of CI testing in Kurtosis Core
	IsKurtosisCoreDevMode bool		`json:"isKurtosisCoreDevMode"`
}

