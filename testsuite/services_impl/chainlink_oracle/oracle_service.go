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

	sessionsEndpoint = "sessions"
	specsEndpoint = "v2/specs"
	ethAccountsEndpoint = "v2/keys/eth"
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
	if chainlinkOracleService.clientWithSession == nil {
		_, err := chainlinkOracleService.StartSession()
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to start session on Oracle.")
		}
	}
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(), chainlinkOracleService.GetOperatorPort(), runsEndpoint)
	response, err := chainlinkOracleService.clientWithSession.Get(urlStr)
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

func (chainlinkOracleService *ChainlinkOracleService) GetEthAccounts() ([]OracleEthereumKey, error) {
	if chainlinkOracleService.clientWithSession == nil {
		_, err := chainlinkOracleService.StartSession()
		if err != nil {
			return nil, stacktrace.Propagate(err, "Failed to start session on Oracle.")
		}
	}
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(), chainlinkOracleService.GetOperatorPort(), ethAccountsEndpoint)
	response, err := chainlinkOracleService.clientWithSession.Get(urlStr)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to get ethereum account info from Oracle.")
	}
	ethereumKeysResponse := new(OracleEthereumKeysResponse)

	err = parseAndLogResponse(response, ethereumKeysResponse)
	if err != nil {
		return nil, stacktrace.Propagate(err, "Failed to parse Oracle response into a struct.")
	}
	return ethereumKeysResponse.Data, nil
}

func (chainlinkOracleService *ChainlinkOracleService) SetJobSpec(oracleContractAddress string) (jobId string, err error) {
	if chainlinkOracleService.clientWithSession == nil {
		_, err := chainlinkOracleService.StartSession()
		if err != nil {
			return "", stacktrace.Propagate(err, "Failed to start session on Oracle.")
		}
	}
	jobSpecJsonStr := generateJobSpec(oracleContractAddress)
	jsonByteArray := []byte(jobSpecJsonStr)
	urlStr := fmt.Sprintf("http://%v:%v/%v",
		chainlinkOracleService.GetIPAddress(), chainlinkOracleService.GetOperatorPort(), specsEndpoint)

	response, err := chainlinkOracleService.clientWithSession.Post(urlStr, "application/json", bytes.NewBuffer(jsonByteArray))
	if err != nil {
		return "", stacktrace.Propagate(err, "Encountered an error trying to set job spec on the Oracle.")
	}
	jobInitiatedResponse := new(OracleJobInitiatedResponse)
	defer response.Body.Close()

	err = parseAndLogResponse(response, jobInitiatedResponse)
	if err != nil {
		return "", stacktrace.Propagate(err, "Failed to parse Oracle response into a struct.")
	}
	return jobInitiatedResponse.Data.Id, nil
}

func (chainlinkOracleService *ChainlinkOracleService) StartSession() (string, error) {
	authJsonStr := fmt.Sprintf("{\"email\":\"%v\", \"password\":\"%v\"}",
		oracleEmail, oraclePassword)
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
	authResp, err := client.Post(urlStr, "application/json", bytes.NewBuffer(authByteArray))
	if err != nil {
		return "", stacktrace.Propagate(err, "Encountered an error trying to authenticate with the oracle service..")
	}
	logrus.Debugf("After starting sessions, cookies look like: %+v", jar)
	chainlinkOracleService.clientWithSession = client
	return authResp.Status, nil
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
				  "type": "HttpGet"
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

		/*
			Operator Type that got removed because EthTx finalization is not working: ,
				{
				  "type": "EthTx"
				}
		 */
}

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