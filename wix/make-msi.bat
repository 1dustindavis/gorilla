@echo off

echo Building Gorilla MSI using WIX

for /f %%i in ('git describe --tags --always --dirty') do set versionString=%%i
echo Version: %versionString%

copy "..\build\gorilla.exe" gorilla.exe 1>NUL

echo Running candle...
call "%wix%bin\candle.exe" gorilla.wxs 1>NUL

echo Running light...
call "%wix%bin\light.exe" -ext WixUtilExtension.dll gorilla.wixobj 1>NUL

echo Cleaning up...
move gorilla.msi gorilla-%versionString%.msi 1>NUL
del gorilla.exe
del gorilla.wixpdb
del gorilla.wixobj

pause