# Build multi-etapa -- la imagen final no lleva el toolchain de Go, solo
# el binario ya compilado (misma idea que docker/php/Dockerfile del
# repo principal: nunca instalar herramientas de build en lo que corre
# de verdad).
FROM golang:1.27 AS build

WORKDIR /src

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# CGO_ENABLED=0: sin esto, algunas dependencias transitivas podrían
# tirar del compilador de C del sistema al compilar -- binario estático
# de verdad, corre en la imagen final mínima sin depender de ninguna
# libc particular. Mismo motivo que ya usa release.yml para el .exe de
# Windows.
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /out/conector-ad ./cmd/conector-ad

# Imagen final -- alpine trae ca-certificates (hace falta para LDAPS,
# ver internal/adldap) y una shell mínima para debug, pero nada del
# toolchain de compilación.
FROM alpine:3.20

RUN apk add --no-cache ca-certificates

COPY --from=build /out/conector-ad /usr/local/bin/conector-ad

# Nunca corre como root -- si alguien logra ejecutar código arbitrario
# adentro (ej. explotando una dependencia), que no sea con privilegios
# de root del contenedor.
RUN addgroup -S conector && adduser -S conector -G conector
USER conector

# "-from-env": el secreto (CONECTOR_AD_PASSWORD) llega por variable de
# entorno -- Docker/Kubernetes ya tienen su propio mecanismo para
# inyectar eso de forma segura (Docker secrets, K8s Secrets, etc.),
# nunca se escribe a un archivo dentro del contenedor. Ver
# internal/config/config.go (CargarDeEnv) para la lista completa de
# variables CONECTOR_*.
ENTRYPOINT ["/usr/local/bin/conector-ad", "-from-env"]
