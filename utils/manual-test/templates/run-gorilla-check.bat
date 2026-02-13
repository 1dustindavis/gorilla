@echo off
setlocal

set "GORILLA_EXE=%ProgramData%\gorilla\bin\gorilla.exe"
set "GORILLA_CONFIG=%ProgramData%\gorilla\config.yaml"

if not exist "%GORILLA_EXE%" (
  echo Missing executable: %GORILLA_EXE%
  exit /b 1
)

if not exist "%GORILLA_CONFIG%" (
  echo Missing config: %GORILLA_CONFIG%
  exit /b 1
)

"%GORILLA_EXE%" -c "%GORILLA_CONFIG%" -C -v %*
exit /b %errorlevel%
