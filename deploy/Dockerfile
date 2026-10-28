FROM node:24-bookworm-slim AS frontend
WORKDIR /app
COPY frontend/package*.json ./
RUN npm ci --ignore-scripts
COPY frontend/ ./
RUN npm run build

FROM golang:1.26-bookworm AS go-build
ENV CGO_ENABLED=0 GOWORK=off
RUN go install github.com/sqlc-dev/sqlc/cmd/sqlc@v1.29.0
WORKDIR /src
COPY shared/ ./shared/
COPY backend/ ./backend/
COPY agent/ ./agent/
RUN cd shared && go mod tidy
RUN cd backend && sqlc generate && go mod tidy && go build -trimpath -o /out/sor-server ./cmd/server
RUN cd agent && sqlc generate && go mod tidy && go build -trimpath -o /out/sor-agent ./cmd/agent

FROM scratch AS agent-artifact
COPY --from=go-build /out/sor-agent /sor-agent

FROM gcr.io/distroless/static-debian12:nonroot AS panel
LABEL org.opencontainers.image.title="SOR Hosting" io.sor.schema-max="1"
COPY --from=go-build /out/sor-server /sor-server
COPY --from=frontend /app/dist /app/frontend
USER 10001:10001
ENV SOR_ADDRESS=0.0.0.0:8080 SOR_DATABASE=/data/panel.db SOR_STATIC_DIR=/app/frontend SOR_AGENT_SOCKET=/run/sor-hosting/agent.sock
EXPOSE 8080
HEALTHCHECK --interval=15s --timeout=5s --start-period=15s --retries=3 CMD ["/sor-server","healthcheck"]
ENTRYPOINT ["/sor-server"]
CMD ["serve"]
