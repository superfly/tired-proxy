# syntax=docker/dockerfile:1
FROM golang:1.19-alpine as build

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY *.go ./

RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /tired-proxy
RUN chmod +x /tired-proxy

FROM scratch

COPY --from=build /tired-proxy /

CMD [ "/tired-proxy" ]