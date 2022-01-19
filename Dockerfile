# syntax=docker/dockerfile:1

FROM golang:1.16-alpine

WORKDIR /app

COPY go.mod ./

RUN go mod download

COPY *.go ./

RUN GOOS=linux GOARCH=amd64 go build -o /tired-proxy

CMD [ "/tired-proxy" ]