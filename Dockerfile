FROM scratch

COPY central-gateway /central-gateway

ENTRYPOINT ["/central-gateway"]

EXPOSE 80
