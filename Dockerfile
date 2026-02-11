FROM golang:1.24-alpine AS build
WORKDIR /app
COPY go.mod ./
COPY *.go ./
COPY lua/ ./lua/
RUN go mod tidy && go mod download
RUN CGO_ENABLED=0 go build -o /factorio-metrics .

FROM scratch
COPY --from=build /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=build /factorio-metrics /factorio-metrics
COPY lua/ /lua/
ENTRYPOINT ["/factorio-metrics"]
