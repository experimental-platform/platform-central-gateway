FROM quay.io/experimentalplatform/ubuntu:latest

COPY dumb-init /dumb-init
COPY platform-central-gateway /central-gateway

ENTRYPOINT ["/dumb-init", "/central-gateway"]

EXPOSE 80

