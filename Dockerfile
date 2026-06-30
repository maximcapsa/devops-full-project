# Single multi-stage Dockerfile shared by all Go services, parameterized by the
# SERVICE build-arg (DRY vs one near-identical Dockerfile per service). The
# final image is distroless static + nonroot. In CI it is built for linux/arm64
# via `docker buildx --platform`; locally compose builds for the host arch.
FROM golang:1.25-alpine AS build
WORKDIR /src

# Cache module downloads.
COPY go.mod go.sum ./
RUN go mod download

# Build the requested service.
COPY . .
ARG SERVICE
RUN test -n "$SERVICE" || (echo "SERVICE build-arg is required" && exit 1)
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/app ./services/${SERVICE}

FROM gcr.io/distroless/static:nonroot
COPY --from=build /out/app /app
USER nonroot:nonroot
ENTRYPOINT ["/app"]
