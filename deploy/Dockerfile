# Builds the single analytics-server binary (section 3: "Production does
# not run Node.js"). Build context is the repo root — see deploy/compose.yaml.
FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /out/analytics-server ./cmd/analytics-server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/analytics-server /analytics-server
COPY migrations /migrations
ENTRYPOINT ["/analytics-server"]
