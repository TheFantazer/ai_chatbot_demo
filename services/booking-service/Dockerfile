FROM golang:1.26-alpine AS build

WORKDIR /src

COPY go.mod ./
COPY contracts ./contracts
COPY services/booking-service ./services/booking-service

RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -o /out/service ./services/booking-service/cmd/booking-service

FROM alpine:3.22

RUN apk add --no-cache ca-certificates \
    && adduser -D -u 10001 appuser
USER appuser

COPY --from=build /out/service /service

ENTRYPOINT ["/service"]
