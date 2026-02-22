@echo off
setlocal EnableExtensions EnableDelayedExpansion

REM gts-comfy-helper - Windows 11 dev launcher
REM Usage:
REM   scripts\dev.cmd
REM Optional env overrides (before running):
REM   set PORT=8877
REM   set COMFYUI_BASE_URL=http://127.0.0.1:8000

where go >nul 2>nul
if errorlevel 1 (
  echo [ERROR] Go no esta instalado o no esta en PATH.
  echo Instala Go y reintenta: https://go.dev/dl/
  exit /b 1
)

if "%HOST%"=="" set "HOST=127.0.0.1"
if "%PORT%"=="" set "PORT=8877"
if "%DATA_DIR%"=="" set "DATA_DIR=%~dp0..\data"
if "%COMFYUI_BASE_URL%"=="" set "COMFYUI_BASE_URL=http://127.0.0.1:8000"
if "%COMFY_POLL_MS%"=="" set "COMFY_POLL_MS=1200"
if "%COMFY_TIMEOUT_MS%"=="" set "COMFY_TIMEOUT_MS=90000"

echo [INFO] Root: %~dp0..
echo [INFO] HOST=%HOST%
echo [INFO] PORT=%PORT%
echo [INFO] DATA_DIR=%DATA_DIR%
echo [INFO] COMFYUI_BASE_URL=%COMFYUI_BASE_URL%
set "APP_URL=http://%HOST%:%PORT%"
set "HEALTH_URL=%APP_URL%/api/health"

goto :run

:run
pushd "%~dp0.."
if errorlevel 1 (
  echo [ERROR] No se pudo entrar al directorio del proyecto.
  exit /b 1
)

echo [INFO] Descargando dependencias...
go mod tidy
if errorlevel 1 (
  echo [ERROR] go mod tidy fallo.
  popd
  exit /b 1
)

where powershell >nul 2>nul
if errorlevel 1 (
  echo [WARN] PowerShell no esta disponible. Se omite auto-open del navegador.
) else (
  echo [INFO] Se abrira el navegador cuando el servidor responda en %HEALTH_URL%
  start "" /B powershell -NoProfile -ExecutionPolicy Bypass -Command "$u='%HEALTH_URL%'; $app='%APP_URL%'; for($i=0; $i -lt 180; $i++){ try { $r=Invoke-WebRequest -UseBasicParsing -Uri $u -TimeoutSec 2; if($r.StatusCode -ge 200 -and $r.StatusCode -lt 500){ Start-Process $app; exit 0 } } catch {} Start-Sleep -Seconds 1 }"
)

echo [INFO] Levantando servidor en %APP_URL%
go run ./cmd/server
set "EXIT_CODE=%ERRORLEVEL%"

popd
exit /b %EXIT_CODE%
