@echo off
setlocal

set "GORILLA_EXE=%ProgramData%\gorilla\bin\gorilla.exe"
set "GORILLA_CONFIG=%ProgramData%\gorilla\config.yaml"
set "EXITCODE=0"

if not exist "%GORILLA_EXE%" (
  echo Missing executable: %GORILLA_EXE%
  set "EXITCODE=1"
  goto end
)

if not exist "%GORILLA_CONFIG%" (
  echo Missing config: %GORILLA_CONFIG%
  set "EXITCODE=1"
  goto end
)

"%GORILLA_EXE%" -c "%GORILLA_CONFIG%" -C -v %*
set "EXITCODE=%errorlevel%"

:end
echo.
pause
exit /b %EXITCODE%
