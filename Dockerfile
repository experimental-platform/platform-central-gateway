FROM dockerregistry.protorz.net/ubuntu:latest

COPY central-gateway /central-gateway

ENTRYPOINT ["/central-gateway", "--port", "80"]

EXPOSE 80
