FROM experimentalplatform/ubuntu:latest

COPY platform-central-gateway /central-gateway

ENTRYPOINT ["dumb-init", "/central-gateway"]

EXPOSE 80
