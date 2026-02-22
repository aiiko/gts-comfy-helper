# gts-comfy-helper

Aplicacion basica en Go + HTML + JavaScript + CSS para generar imagenes en ComfyUI con live preview.

## Features

- UI dark moderna (prompt + boton generar + preview)
- Live preview durante generacion
- Polling de preview cada 500ms
- Imagen final completa al terminar
- Configuracion persistente en SQLite:
  - `positive_tags`
  - `negative_tags`
  - `last_aspect_ratio`
- Selector opcional de camera angle cargado desde JSON embebido (`internal/server/camera_options.json`)
- `positive_tags` se anteponen al prompt del usuario
- Orden actual del prompt final (backend): `positive_tags`, `character_definition`, `prompt`, `art_style`, `body_framing`, `camera_selector`
- Workflow JSON basado en `GTS-VN-Sim`

## Variables de entorno

- `HOST` (default `127.0.0.1`)
- `PORT` (default `8877`)
- `DATA_DIR` (default `./data`)
- `DB_PATH` (opcional, default `${DATA_DIR}/gts-comfy-helper.sqlite`)
- `COMFYUI_BASE_URL` (default `http://127.0.0.1:8000`)
- `COMFY_POLL_MS` (default `1200`)
- `COMFY_TIMEOUT_MS` (default `90000`)

## Ejecutar

```bash
go mod tidy
go run ./cmd/server
```

Luego abrir:

- http://127.0.0.1:8877

## API

- `GET /api/settings`
- `PUT /api/settings`
- `GET /api/camera-options`
- `POST /api/generate`
- `GET /api/jobs/{id}`
- `GET /api/jobs/{id}/preview?since_seq=0`
- `GET /assets/{file}`

## Nota de workflow

Se reutiliza el workflow de `../GTS-VN-Sim/backend/internal/gen/t2i/illustration_workflow_anima_api.json`, copiado localmente en:

- `internal/comfy/workflow.json`
