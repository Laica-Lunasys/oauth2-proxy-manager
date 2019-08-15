FROM golang:1.12.6 AS build
WORKDIR /go/src/github.com/Laica-Lunasys/oauth2-proxy-manager

ENV GOOS linux
ENV CGO_ENABLED 0

RUN go get -u -v github.com/golang/dep/cmd/dep
ADD Gopkg.lock Gopkg.lock
ADD Gopkg.toml Gopkg.toml
RUN dep ensure -v --vendor-only
COPY . .
RUN go build -a -installsuffix cgo -v -o create-oauth2-proxy ./showcase/create-oauth2-proxy/main.go

FROM alpine
WORKDIR /app

EXPOSE 8080
COPY --from=build /go/src/github.com/Laica-Lunasys/oauth2-proxy-manager/create-oauth2-proxy /app/

ENTRYPOINT ["/app/create-oauth2-proxy"]
