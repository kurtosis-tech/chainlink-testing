package execution_impl

type ChainlinkTestsuiteArgs struct {
	GethServiceImage	string 			`json:"gethServiceImage"`

	// Indicates that this testsuite is being run as part of CI testing in Kurtosis Core
	IsKurtosisCoreDevMode bool		`json:"isKurtosisCoreDevMode"`
}

