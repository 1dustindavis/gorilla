@echo off
setlocal

set "PS1_PATH=%~dp0..\..\integration\windows\run-release-integration.ps1"
set "GORILLA_EXE=%ProgramData%\gorilla\bin\gorilla.exe"
set "WORK_ROOT=%TEMP%\gorilla-release-integration"

if not "%~1"=="" (
  set "GORILLA_EXE=%~1"
)

if not "%~2"=="" (
  set "WORK_ROOT=%~2"
)

if not exist "%PS1_PATH%" (
  echo Missing integration script: %PS1_PATH%
  exit /b 1
)

echo Running Windows release integration test
echo Gorilla exe: %GORILLA_EXE%
echo Work root : %WORK_ROOT%
echo.

powershell -NoProfile -NoLogo -NonInteractive -ExecutionPolicy Bypass -File "%PS1_PATH%" -GorillaExePath "%GORILLA_EXE%" -WorkRoot "%WORK_ROOT%"
set "EXITCODE=%errorlevel%"
echo.
pause
exit /b %EXITCODE%
