# Tired proxy

[![Docker image size](https://img.shields.io/docker/image-size/tiesv/tired-proxy?sort=date "Docker image size") ](https://hub.docker.com/r/tiesv/tired-proxy)

_Altered version of superfly/tired-proxy_

An HTTP proxy that exits after a settable idle time. Useful for creating _scale to zero_ Docker containers. Includes the feature to wait for the port of the origin server to be in use.

## Usage
Inside your container script, start your web application and fork the process with Tired proxy.
```bash
./my-web-server & /tired-proxy --origin=http://localhost:3000
```
The proxy is now served at port 8080

### CLI options

|Option|type|default|description|
|---|---|---|---|
|`idle-time`|int|60|idle time in seconds after which the application shuts down, if no requests where received|
|`origin`|string|http://localhost|the origin host to which the requests are forwarded|
|`port`|string|8080|port at which the proxy server listens for requests|
|`wait-for-port`|int|0|maximum time in seconds to wait before the origin servers port is in use before starting the proxy server|
|`verbose`|bool|false|verbose logging output|

## Get the docker image
```bash
docker pull tiesv/tired-proxy:latest
```