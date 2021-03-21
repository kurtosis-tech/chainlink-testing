FROM node:10-alpine

RUN apk update && \
    apk upgrade && \
    apk add git && \
    apk add bash && \
    apk add python

RUN npm install -g truffle
RUN npm install -g ethers
RUN mkdir -p /chainlink
WORKDIR /chainlink

### Source: https://github.com/smartcontractkit/box
### All of our smartcontract deployment and manipulation comes from this default truffle box.
RUN truffle unbox smartcontractkit/box
RUN yarn && yarn compile

COPY . .

ENTRYPOINT /bin/sh

