# Verificación

## Comandos

`make check` genera sqlc, ejecuta tests Go de los tres módulos, go vet, tests del cliente, typecheck y build de producción. `make fmt` aplica gofmt. `python3 tools/check-schema.py` prueba el esquema y sus restricciones usando SQLite real. `npm run test:e2e --prefix frontend` verifica el build con Playwright y una API controlada, sin necesitar una VPS real.

## Entorno de desarrollo de esta entrega

Node/npm estaban disponibles. Las dependencias se instalaron offline desde una copia temporal de la caché; no se alteró la caché original. El frontend pasó TypeScript y Vite. Se verificaron nueve casos SQLite (incluida preparación de todas las consultas de ambos módulos) y cuatro casos del cliente HTTP. Las pruebas Go de cookies, expiración, CSRF, cambio de contraseña, políticas y reconciliación están incluidas para ejecutarse con el toolchain.

Los tests de API del cliente se ejecutan con `node:test` directamente, para obtener el número real de casos incluso en entornos con aislamiento de procesos limitado.

Go y sqlc no estaban instalados; DNS de terminal impedía descargarlos. El daemon Docker y los sockets para Vite/Chromium estaban bloqueados. Por ello **no se afirman compilaciones o tests Go, una imagen Docker construida, pruebas E2E de navegador ni integraciones reales exitosas en este entorno**. Los paquetes sqlc y go.sum se generan al resolver dependencias con el bootstrap. CI repite esas comprobaciones con toolchain completo antes de publicar una imagen.

## Prueba previa a producción

En un host Linux desechable con Nginx, Certbot de staging, Docker y Prometheus:

1. Instalar agente/panel, crear admin, comprobar login/logout/CSRF y acceso directo a rutas.
2. Probar 375/768/1024/1440 px, teclado, foco de diálogos, zoom 200 %, reduced-motion y contraste.
3. Crear WordPress con DNS correcto e incorrecto; comprobar readiness DB, puerto web loopback, DB sin publicar y reintento SSL.
4. Denegar acciones/logs de contenedores no autorizados y todos los recursos protegidos. Probar nombres parecidos e IDs reemplazados.
5. Forzar fallo de nginx -t y comprobar rollback; probar renovación selectiva, DNS mixto y Certbot no elegible.
6. Crear contenido y DB, respaldar, modificar, restaurar y verificar ambos. Probar checksum inválido, espacio insuficiente, symlinks/traversal y fallo del snapshot.
7. Matar API/agente durante una operación y durante mantenimiento/cutover. Confirmar recuperación y que no se repiten efectos inciertos.
8. Recrear contenedor de API y comprobar persistencia. Probar release con imagen no ready y rollback compatible.

Estos pasos deben quedar registrados antes de habilitar las operaciones sobre sitios reales. Los fixtures del navegador se limitan al código de pruebas y nunca se habilitan como modo de producción.
