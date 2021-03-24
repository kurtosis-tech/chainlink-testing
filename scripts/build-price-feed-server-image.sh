PRICE_FEED_SERVER_IMAGE_TAG="kurtosistech/chainlink-price-feed-server:latest"

docker build testsuite/services_impl/price_feed_server/docker/ -t "${PRICE_FEED_SERVER_IMAGE_TAG}"
docker push "${PRICE_FEED_SERVER_IMAGE_TAG}"