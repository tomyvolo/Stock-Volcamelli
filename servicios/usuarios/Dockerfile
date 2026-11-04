# Primera etapa: compila el servicio.
FROM golang:1.22-alpine AS build

WORKDIR /app

# Copiar dependencias primero permite reutilizar la caché de Docker.
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 go build -o /usuarios .

# Segunda etapa: contiene solo el binario, sin compilador ni código fuente.
FROM alpine:3.21
COPY --from=build /usuarios /usuarios
EXPOSE 8081
CMD ["/usuarios"]
