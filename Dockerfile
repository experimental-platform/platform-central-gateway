FROM quay.io/experimentalplatform/ubuntu:latest

COPY dumb-init /dumb-init
COPY platform-central-gateway /central-gateway
COPY 502.html /502.html
COPY entrypoint.sh /entrypoint

ENTRYPOINT ["/entrypoint"]

EXPOSE 80
EXPOSE 443

