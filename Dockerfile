FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY *.go ./
COPY lua/ ./lua/
RUN go mod tidy -e
RUN CGO_ENABLED=0 go build -o /factorio-metrics .

FROM alpine:3.21
COPY --from=build /factorio-metrics /factorio-metrics
COPY lua/ /lua/
ENTRYPOINT ["/factorio-metrics"]
