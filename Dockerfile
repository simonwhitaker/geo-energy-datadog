# syntax=docker/dockerfile:1

FROM golang:1.21.4-alpine AS build

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . ./
RUN CGO_ENABLED=0 GOOS=linux go build -o /geo-energy-datadog

FROM alpine:3.18.2

WORKDIR /

COPY --from=build /geo-energy-datadog /geo-energy-datadog

ENTRYPOINT [ "/geo-energy-datadog" ]
