FROM node:10-alpine

RUN apk update && \
    apk upgrade && \
    apk add git

RUN git clone "https://github.com/smartcontractkit/LinkToken.git"
WORKDIR LinkToken

RUN yarn && yarn compile

ENTRYPOINT /bin/sh

