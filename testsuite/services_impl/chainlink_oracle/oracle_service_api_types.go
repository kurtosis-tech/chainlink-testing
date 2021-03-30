package chainlink_oracle

// ====================================================================================================
//                                         Get Runs
// ====================================================================================================
type RunsResponse struct {
	Data []Run `json:"data"`
}

type Run struct {
	Type string `json:"type"`
	Attributes RunAttributes `json:"attributes"`
}

type RunAttributes struct {
	Id string `json:"id"`
	JobId string `json:"jobId"`
	Status string `json:"status"`
	TaskRuns []TaskRun `json:"taskRuns"`
	Initiator Initiator `json:"initiator"`
	Payment string `json:"payment"`
}

type TaskRun struct {
	Id string `json:"id"`
	Status string `json:"status"`
}


// ====================================================================================================
//                                         Get Runs
// ====================================================================================================
type Initiator struct {
	Id int `json:"id"`
	JobSpecId string `json:"jobSpecId"`
}


// ====================================================================================================
//                                       Get Eth Keys
// ====================================================================================================
type OracleEthereumKeysResponse struct {
	Data []OracleEthereumKey `json:"data"`
}

type OracleEthereumKey struct {
	Type string `json:"type"`
	Attributes EthereumKeyAttributes `json:"attributes"`
}

type EthereumKeyAttributes struct {
	Address string `json:"address"`
	EthBalance string `json:"ethBalance"`
	LinkBalance string `json:"linkBalance"`
}



// ====================================================================================================
//                                       Get P2P Keys
// ====================================================================================================
type OraclePeerToPeerKeysResponse struct {
	Data []OraclePeerToPeerKey `json:"data"`
}

type OraclePeerToPeerKey struct {
	Type string `json:"type"`
	Attributes PeerToPeerKeyAttributes `json:"attributes"`
}

type PeerToPeerKeyAttributes struct {
	PeerId string `json:"peerId"`
	PublicKey string `json:"publicKey"`
}




// ====================================================================================================
//                                       Get OCR Key Bundles
// ====================================================================================================
type OracleOcrKeyBundlesResponse struct {
	Data []OracleOcrKeyBundle `json:"data"`
}

type OracleOcrKeyBundle struct {
	Type string `json:"type"`
	Attributes OcrKeyBundleAttributes `json:"attributes"`
}

type OcrKeyBundleAttributes struct {
	Id string `json:"id"`
	OnChainSigningAddress string `json:"onChainSigningAddress"`
	OffChainPublicKey string `json:"offChainPublicKey"`
	ConfigPublicKey string `json:"configPublicKey"`
}


// ====================================================================================================
//                                       Initiate Job
// ====================================================================================================
type OracleJobInitiatedResponse struct {
	Data OracleJobInitiatedData `json:"data"`
}

type OracleJobInitiatedData struct {
	Id string `json:"id"`
}
