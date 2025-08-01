# stage 1 Build blobstream-ops binary
FROM --platform=$BUILDPLATFORM docker.io/golang:1.24.3-alpine3.20 as builder

ARG TARGETOS
ARG TARGETARCH

ENV CGO_ENABLED=0
ENV GO111MODULE=on

RUN apk update && apk --no-cache add make gcc musl-dev git bash

COPY . /blobstream-ops
WORKDIR /blobstream-ops
RUN uname -a &&\
    CGO_ENABLED=${CGO_ENABLED} GOOS=${TARGETOS} GOARCH=${TARGETARCH} \
    make build

# final image
FROM docker.io/alpine:3.22.1

ARG UID=10001
ARG USER_NAME=celestia

ENV CELESTIA_HOME=/home/${USER_NAME}

# hadolint ignore=DL3018
RUN apk update && apk add --no-cache \
        bash \
        curl \
        jq \
    # Creates a user with $UID and $GID=$UID
    && adduser ${USER_NAME} \
        -D \
        -g ${USER_NAME} \
        -h ${CELESTIA_HOME} \
        -s /sbin/nologin \
        -u ${UID}

COPY --from=builder /blobstream-ops/build/blobstream-ops /bin/blobstream-ops
COPY --chown=${USER_NAME}:${USER_NAME} docker/entrypoint.sh /opt/entrypoint.sh

USER ${USER_NAME}

# p2p port
EXPOSE 30000

ENTRYPOINT [ "/bin/bash", "/opt/entrypoint.sh" ]
