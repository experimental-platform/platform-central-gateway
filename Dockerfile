FROM quay.io/experimentalplatform/ubuntu:latest

COPY dumb-init /dumb-init
COPY platform-central-gateway /central-gateway
COPY entrypoint.sh /entrypoint

ENTRYPOINT ["/entrypoint"]

EXPOSE 80
EXPOSE 443

