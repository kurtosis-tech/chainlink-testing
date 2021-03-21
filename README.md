[WIP] Chainlink - Geth Kurtosis Testsuite
=====================
This is a work in progress, currently in development as part of the first Chainlink/Kurtosis grant: https://blog.chain.link/kurtosis-awarded-a-grant-to-build-testing-platform-for-chainlink-oracle-networks/ .

## Architecture

The testnet is composed of `geth` ethereum nodes with manual peer connections.
The Chainlink truffle box (https://github.com/smartcontractkit/box) is used in a 
custom helper Docker container (testsuite/services_impl/chainlink_contract_deployer) 
to deploy $LINK token definition contracts and
utility contracts to the testnet.

## To Run 

To build the contract_deployer Docker image, run `bash scripts/build-chainlink-contract-deployer-image.sh`.

To run the testsuite, run `bash scripts/build-and-run.sh all`. To see help information, run `bash scripts/build-and-run.sh help'`

## Testsuite Setup Steps

1. Spin up a private ethereum testnet in Kurtosis.
2. Start a postgres database, which will be used by the Chainlink Oracle service.
3. Spin up a container containing Truffle, and the Chainlink Truffle Box (https://github.com/smartcontractkit/box).
4. Use the Chainlink Truffle container to deploy a set of standard smart contracts required by the Chainlink Oracle (ex. $LINK definition, Oracle contract, example end-user contract)
5. Fund ethereum accounts generated by the Oracle service, so that it can fuel transactions on-chain.
6. Fund $LINK accounts generated by the truffle container deployment so that they can request data from the Chainlink oracle.
7. Configure a job on the Oracle which can request data via HTTPGet and parse it (https://docs.chain.link/docs/job-specifications) 
8. Use the script `scripts/request-data.js` on the Truffle container to request data from the Oracle.
9. Use the Oracle HTTP endpoints to verify that the job has completed successfully.