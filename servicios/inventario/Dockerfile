# Primera etapa: compila el main.go y sus paquetes con la misma versión de Go del práctico.
FROM golang:1.22-alpine AS build

# El directorio de trabajo queda dentro de la imagen, no modifica el host.
WORKDIR /app

# Copiar dependencias primero permite reutilizar la caché de Docker.
COPY go.mod go.sum ./
RUN go mod download

# Copiar el código y compilar un binario Linux estático.
COPY . .
RUN CGO_ENABLED=0 go build -o /inventario .

# Segunda etapa: contiene solo el binario, sin compilador ni código fuente.
FROM alpine:3.21
COPY --from=build /inventario /inventario
EXPOSE 8082
CMD ["/inventario"]
