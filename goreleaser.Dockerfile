FROM alpine:3.14 AS runtime
COPY juno /juno
ENTRYPOINT ["/juno"]