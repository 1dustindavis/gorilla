@echo off
setlocal

set "DEFAULT_BASE_URL=@DEFAULT_BASE_URL@"

if "%~1"=="" (
  set "BASE_URL=%DEFAULT_BASE_URL%"
) else (
  set "BASE_URL=%~1"
  shift
)

echo Using Base URL: %BASE_URL%

powershell -NoProfile -ExecutionPolicy Bypass -File "%~dp0bootstrap-vm-full.ps1" -BaseUrl "%BASE_URL%" -NoPause %*
set "EXITCODE=%errorlevel%"
echo.
pause
exit /b %EXITCODE%
