FROM golang:1.13.15 AS golang
ADD . /app
WORKDIR /app
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 GO11MODULE=on go build -a -o /main .
	
FROM debian:stretch-slim
COPY --from=golang /main /kubernetes-overlay-threshold
RUN chmod +x /kubernetes-overlay-threshold && \
    apt-get update && \
    apt-get upgrade -y && \
    apt-get install -y \
        iputils-ping