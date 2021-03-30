package chainlink_oracle

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/kurtosis-tech/kurtosis-libs/golang/lib/services"
	"github.com/palantir/stacktrace"
	"github.com/sirupsen/logrus"
	"golang.org/x/net/publicsuffix"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"time"
)

const (
	isAvailableDialTimeout = time.Second

	apiVersion = "v2"

	sessionsEndpoint = "sessions"
	specsEndpoint = "v2/specs"
	keysEndpoint = "v2/keys"
	ethKeyEndpointSuffix = "eth"
	ocrKeyEndpointSuffix = "ocr"
	peerToPeerIdEndpointSuffix = "p2p"

	runsEndpoint = "v2/runs"
)

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

type Initiator struct {
	Id int `json:"id"`
	JobSpecId string `json:"jobSpecId"`
}


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

type OracleJobInitiatedResponse struct {
	Data OracleJobInitiatedData `json:"data"`
}

type OracleJobInitiatedData struct {
	Id string `json:"id"`
}

type ChainlinkOracleService struct {
	serviceCtx *services.ServiceContext
	clientWithSession *http.Client
	sessionCookieJar *cookiejar.Jar
}

func NewChainlinkOracleService(serviceCtx *services.ServiceContext) *ChainlinkOracleService {
	return &ChainlinkOracleService{serviceCtx: serviceCtx}
}

func (chainlinkOracleService *ChainlinkOracleService) GetOperatorPort() int {
	return operatorUiPort
}

func (chainlinkOracleService *ChainlinkOracleService) GetIPAddress() string {
	return chainlinkOracleService.serviceCtx.GetIPAddress()
}

func (chainlinkOracleService *ChainlinkOracleService) GetRuns() ([]Run, error) {
	client, err := chainlinkOracleService.getOrCreateClientWithSession()
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the oracle session client")
	}
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(), chainlinkOracleService.GetOperatorPort(), runsEndpoint)
	response, err := client.Get(urlStr)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to get runs information from Oracle.")
	}
	runsResponse := new(RunsResponse)
	err = parseAndLogResponse(response, runsResponse)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to parse Oracle response into a struct.")
	}
	return runsResponse.Data, nil
}

/*
func (chainlinkOracleService *ChainlinkOracleService) GetEthAccounts() ([]OracleEthereumKey, error) {
	client, err := chainlinkOracleService.getOrCreateClientWithSession()
	if err != nil {
		return nil, stacktrace.Propagate(err, "An error occurred getting the oracle session client")
	}
}
 */

func (chainlinkOracleService *ChainlinkOracleService) SetJobSpec(
		oracleContractAddress string) (jobId string, err error) {
	client, err := chainlinkOracleService.getOrCreateClientWithSession()
	if err != nil {
		return "", stacktrace.Propagate(err, "An error occurred getting the oracle session client")
	}

	// Get transmitter key
	/*
	url := chainlinkOracleService.getApiRequestUrl(fmt.Sprintf(
		"%v/%v",
		keysEndpoint,
		ethKeyEndpointSuffix))
	jobSpecsResponse, err := client.Get(url)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to get Ethereum account info from oracle")
	}
	ethereumKeys := new(OracleEthereumKeysResponse)
	err = parseAndLogResponse(jobSpecsResponse, ethereumKeys)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to parse Ethereum account info response")
	}
	// TODO which key(s) do we use??

	// Get P2P ID
	peerToPeerIdUrl := chainlinkOracleService.getApiRequestUrl(fmt.Sprintf("%v/%v", keysEndpoint, peerToPeerIdEndpointSuffix))
	peerToPeerIdResponse, err := client.Get(peerToPeerIdUrl)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to get peer-to-peer ID from oracle")
	}
	logrus.Debugf("Peer-to-peer ID response: %v", peerToPeerIdResponse)

	// TODO which response object should we use???
	ethereumKeys := new(OracleEthereumKeysResponse)
	err = parseAndLogResponse(jobSpecsResponse, ethereumKeys)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to parse Ethereum account info response")
	}

	// TODO get the key bundle ID
	 */

	jobSpecJsonStr := generateJobSpec(oracleContractAddress)
	jsonByteArray := []byte(jobSpecJsonStr)
	url := fmt.Sprintf(
		"http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(),
		chainlinkOracleService.GetOperatorPort(),
		specsEndpoint)

	resp, err := client.Post(url, "application/json", bytes.NewBuffer(jsonByteArray))
	if err != nil {
		return "", stacktrace.Propagate(err, "Encountered an error trying to set job spec on the Oracle.")
	}
	jobInitiatedResponse := new(OracleJobInitiatedResponse)
	defer resp.Body.Close()

	err = parseAndLogResponse(resp, jobInitiatedResponse)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to parse Oracle response into a struct.")
	}
	return jobInitiatedResponse.Data.Id, nil
}

// ===========================================================================================
//                              Service interface methods
// ===========================================================================================

func (chainlinkOracleService *ChainlinkOracleService) IsAvailable() bool {
	conn, err := net.DialTimeout("tcp",
		net.JoinHostPort(chainlinkOracleService.GetIPAddress(), strconv.Itoa(operatorUiPort)), isAvailableDialTimeout)
	if err != nil {
		return false
	}
	if conn == nil {
		return false
	}
	defer conn.Close()
	return true
}


// ===========================================================================================
//                              Private helper functions
// ===========================================================================================
// TODO Push this into a supplier struct, so that users don't accidentally use the uninitialized clientWithSession
//  property on the struct
func (chainlinkOracleService *ChainlinkOracleService) getOrCreateClientWithSession() (*http.Client, error) {
	if chainlinkOracleService.clientWithSession != nil {
		return chainlinkOracleService.clientWithSession, nil
	}

	authJsonStr := fmt.Sprintf(
		"{\"email\":\"%v\", \"password\":\"%v\"}",
		oracleEmail,
		oraclePassword)
	authByteArray := []byte(authJsonStr)
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(), chainlinkOracleService.GetOperatorPort(), sessionsEndpoint)
	// Create new cookiejar for holding cookies
	jar, _ := cookiejar.New(&cookiejar.Options{PublicSuffixList: publicsuffix.List})

	// Create new http client with predefined options
	client := &http.Client{
		Jar:     jar,
		Timeout: time.Second * 60,
	}
	_, err := client.Post(urlStr, "application/json", bytes.NewBuffer(authByteArray))
	if err != nil {
		return nil, stacktrace.Propagate(err, "Encountered an error trying to authenticate with the oracle service..")
	}
	logrus.Debugf("After starting sessions, cookies look like: %+v", jar)
	chainlinkOracleService.clientWithSession = client
	return client, nil
}

func (chainlinkOracleService *ChainlinkOracleService) getApiRequestUrl(endpoint string) string {
	return fmt.Sprintf(
		"http://%v:%v/%v/%v",
		chainlinkOracleService.GetIPAddress(),
		chainlinkOracleService.GetOperatorPort(),
		apiVersion,
		endpoint)
}

func generateJobSpec(oracleContractAddress string) string {
	return fmt.Sprintf(`{
		  "initiators": [
			{
			  "type": "RunLog",
			  "params": { "address": "%v" }
			}
		  ],
		  "tasks": [
				{
				  "type": "HttpGetWithUnrestrictedNetworkAccess"
				},
				{
				  "type": "JsonParse"
				},
				{
				  "type": "Multiply"
				},
				{
				  "type": "EthInt256"
				},
				{
				  "type": "EthTx"
				}
		  ]
		}`, oracleContractAddress)
}


/*
func generateJobSpec(
			oracleContractAddress string,
			bootstrapIpAddr string,
			bootstrapPeerToPeerId string,
			nodePeerToPeerId string,
			nodeOcrKeyBundleId string,
			nodeEthTransmitterAddress string) string {
		// TODO Add an EthInt256 step to this??
		return fmt.Sprintf(`
	type               = "offchainreporting"
	schemaVersion      = 1
	contractAddress    = "%v"
	p2pPeerID          = "%v"
	p2pBootstrapPeers  = [
		"/dns4/%v/tcp/1234/p2p/%v",
	]
	isBootstrapPeer    = false
	keyBundleID        = "%v"
	monitoringEndpoint = "chain.link:4321"
	transmitterAddress = "%v"
	observationTimeout = "10s"
	blockchainTimeout  = "20s"
	contractConfigTrackerSubscribeInterval = "2m"
	contractConfigTrackerPollInterval = "1m"
	contractConfigConfirmations = 3
	observationSource = """
		// data source 1
		ds1          [type=http method=POST url="(http://external-adapter:6633)" requestData="{}"];
		ds1_parse    [type=jsonparse path="data,result"];
		ds1_multiply [type=multiply times=10];

		ds1 -> ds1_parse -> ds1_multiply -> answer;
		answer [type=median];
	"""`,
			oracleContractAddress,
			nodePeerToPeerId,
			bootstrapIpAddr,
			bootstrapPeerToPeerId,
			nodeOcrKeyBundleId,
			nodeEthTransmitterAddress)
}
*/

/*
	Parses an HTTP response into the target struct, while also logging it as a string to help develop and debug.
 */
func parseAndLogResponse(resp *http.Response, targetStruct interface{}) error{
	var teeBuf bytes.Buffer
	tee := io.TeeReader(resp.Body, &teeBuf)
	bodyBytes, err := ioutil.ReadAll(tee)
	if err != nil {
		return stacktrace.Propagate(err, "Error parsing Oracle response into bytes.")
	}
	bodyString := string(bodyBytes)
	logrus.Debugf("Response from Oracle: %v", bodyString)


	err = json.NewDecoder(&teeBuf).Decode(targetStruct)
	if err != nil {
		return stacktrace.Propagate(err, "Error parsing Oracle response into a struct.")
	}
	logrus.Debugf("Response from Chainlink Oracle: %+v", targetStruct)
	return nil
}