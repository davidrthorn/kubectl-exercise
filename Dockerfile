FROM alpine:3.6

COPY bin/server bin/
EXPOSE 8080

ENTRYPOINT [ "/bin/k8-exercise" ]
