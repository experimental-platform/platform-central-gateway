FROM experimentalplatform/ubuntu:latest

COPY platform-central-gateway /central-gateway

ENTRYPOINT ["/central-gateway"]

EXPOSE 80
