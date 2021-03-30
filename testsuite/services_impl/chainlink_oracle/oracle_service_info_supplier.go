package chainlink_oracle

// Encapsulates information about an oracle node for use when configuring an OCR contract
type ocrInfo struct {
	peerToPeerId string
	keyBundleId string
	transmitterAddr string
}

// This struct is responsible for lazily supplying & caching OCR info about an Oracle
type oracleServiceOcrInfoSupplier struct {
	cachedOcrInfo *ocrInfo
	oracleService *ChainlinkOracleService
}

func newOracleServiceInfoSupplier(oracleService *ChainlinkOracleService) *oracleServiceOcrInfoSupplier {
	return &oracleServiceOcrInfoSupplier{
		cachedOcrInfo: nil,
		oracleService: oracleService,
	}
}
