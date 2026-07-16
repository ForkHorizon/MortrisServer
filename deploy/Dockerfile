# Builds the single analytics-server binary (section 3: "Production does
# not run Node.js" — Node is a build-time-only dependency here, never
# shipped in the final image). Build context is the repo root — see
# deploy/compose.yaml.

FROM node:22 AS frontend
WORKDIR /src/dashboard
COPY dashboard/package.json dashboard/package-lock.json ./
RUN npm ci
COPY dashboard/ ./
RUN npm run build

FROM golang:1.26 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
COPY --from=frontend /src/dashboard/dist ./dashboard/dist
RUN CGO_ENABLED=0 go build -o /out/analytics-server ./cmd/analytics-server

FROM gcr.io/distroless/static-debian12
COPY --from=build /out/analytics-server /analytics-server
COPY migrations /migrations
ENTRYPOINT ["/analytics-server"]
