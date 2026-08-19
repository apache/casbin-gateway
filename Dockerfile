FROM node:20 AS FRONT
WORKDIR /web
COPY ./web .
RUN yarn install --frozen-lockfile --network-timeout 1000000 && yarn build

FROM golang:1.20.12 AS BACK
WORKDIR /go/src/casbin-gateway
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-w -s" -o server .

FROM alpine:latest AS STANDARD
LABEL MAINTAINER="https://caswaf.org/"

COPY --from=BACK /go/src/casbin-gateway/ ./
RUN mkdir -p web/build && apk add --no-cache bash coreutils tzdata
COPY --from=FRONT /web/build /web/build
# Holds the SQLite database at /data/casbin-gateway.db.
VOLUME /data
ENTRYPOINT ["./server"]
