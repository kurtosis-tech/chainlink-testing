FROM node:10-alpine

RUN apk update && \
    apk upgrade && \
    apk add git && \
    apk add bash && \
    apk add python

RUN git clone "https://github.com/smartcontractkit/LinkToken.git"
RUN npm install -g truffle
RUN mkdir -p /chainlink
WORKDIR /chainlink
RUN truffle unbox smartcontractkit/box
RUN yarn
RUN truffle compile
WORKDIR /LinkToken

RUN yarn && yarn compile

ENTRYPOINT /bin/sh

