@echo off

echo Building Gorilla MSI using WIX

for /f %%i in ('git describe --tags --always --dirty') do set versionString=%%i
echo Version: %versionString%

if "%PRODUCT_VERSION%"=="" (
  set "productVersion=%versionString%"
) else (
  set "productVersion=%PRODUCT_VERSION%"
)

if "%productVersion:~0,1%"=="v" set "productVersion=%productVersion:~1%"
for /f "tokens=1 delims=-+" %%i in ("%productVersion%") do set "productVersion=%%i"
echo ProductVersion: %productVersion%

copy "..\build\gorilla.exe" gorilla.exe 1>NUL

echo Running candle...
call "%wix%bin\candle.exe" -dProductVersion=%productVersion% gorilla.wxs 1>NUL

echo Running light...
call "%wix%bin\light.exe" -ext WixUtilExtension.dll gorilla.wixobj 1>NUL

echo Cleaning up...
move gorilla.msi gorilla-%versionString%.msi 1>NUL
del gorilla.exe
del gorilla.wixpdb
del gorilla.wixobj

pause
