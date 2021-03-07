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
