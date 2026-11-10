# Primera etapa: compila el servicio. El SDK de MCP pide Go 1.25 (ADR-018).
FROM golang:1.25-alpine AS build

WORKDIR /app

# Copiar dependencias primero permite reutilizar la caché de Docker.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /mcp .

# Segunda etapa: contiene solo el binario, sin compilador ni código fuente.
FROM alpine:3.21
COPY --from=build /mcp /mcp
EXPOSE 8086
CMD ["/mcp"]
