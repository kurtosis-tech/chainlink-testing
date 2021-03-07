CONTRACT_DEPLOYER_IMAGE_TAG="kurtosistech/chainlink-contract-deployer:latest"

docker build testsuite/services_impl/chainlink_contract_deployer/docker/ -t "${CONTRACT_DEPLOYER_IMAGE_TAG}"
docker push "${CONTRACT_DEPLOYER_IMAGE_TAG}"