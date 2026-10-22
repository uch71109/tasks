FROM golang:1.24-alpine AS build
WORKDIR /src

# Download dependencies first so they are cached independently of source changes.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/tasks ./main

FROM gcr.io/distroless/static-debian12:nonroot
COPY --from=build /out/tasks /tasks
EXPOSE 8080
ENTRYPOINT ["/tasks"]
