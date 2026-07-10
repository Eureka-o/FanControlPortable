Unicode true

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "FileFunc.nsh"
!include "StrFunc.nsh"

${Using:StrFunc} StrStr

!define APP_NAME "OMEN Fan Plugin"
!define COMPANY_NAME "Eureka-o"
!define PLUGIN_ID "omen-fan"
!define PLUGIN_VERSION "0.1.0"
!define PLUGIN_VERSION_NUMERIC "0.1.0.0"
!define PLUGIN_UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\OmenFanPlugin"
!define FANCONTROL_UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Eureka-o FanControl"

Name "${APP_NAME}"
OutFile "..\..\..\build\bin\omen-fan-setup.exe"
InstallDir "$PROGRAMFILES64\FanControl"
RequestExecutionLevel admin
ShowInstDetails show
ShowUninstDetails show

VIProductVersion "${PLUGIN_VERSION_NUMERIC}"
VIFileVersion "${PLUGIN_VERSION_NUMERIC}"
VIAddVersionKey /LANG=1033 "ProductName" "${APP_NAME}"
VIAddVersionKey /LANG=1033 "CompanyName" "${COMPANY_NAME}"
VIAddVersionKey /LANG=1033 "ProductVersion" "${PLUGIN_VERSION}"
VIAddVersionKey /LANG=1033 "FileVersion" "${PLUGIN_VERSION}"
VIAddVersionKey /LANG=1033 "FileDescription" "${APP_NAME} Installer"
VIAddVersionKey /LANG=1033 "LegalCopyright" "${COMPANY_NAME}"

!define MUI_ABORTWARNING
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "English"

Var PluginDir
Var DetectDir
Var DetectExit
Var DetectOutput
Var DetectNeedWarning

Function .onInit
    SetRegView 64
    ReadRegStr $0 HKLM "${FANCONTROL_UNINSTALL_KEY}" "InstallLocation"
    ${If} $0 != ""
        StrCpy $INSTDIR $0
    ${Else}
        StrCpy $INSTDIR "$PROGRAMFILES64\FanControl"
    ${EndIf}
FunctionEnd

Function ValidateFanControlRoot
    ${If} ${FileExists} "$INSTDIR\FanControl.exe"
        Return
    ${EndIf}
    ${If} ${FileExists} "$INSTDIR\FanControl Core.exe"
        Return
    ${EndIf}
    ${If} ${FileExists} "$INSTDIR\FanControlPortable.exe"
        Return
    ${EndIf}
    ${If} ${FileExists} "$INSTDIR\FanControlPortable Core.exe"
        Return
    ${EndIf}

    MessageBox MB_ICONSTOP|MB_OK "Please choose an existing FanControl root folder."
    Abort
FunctionEnd

Function WarnIfOmenUnsupported
    StrCpy $DetectNeedWarning "1"
    InitPluginsDir
    StrCpy $DetectDir "$PLUGINSDIR\omen-fan-detect"

    CreateDirectory "$DetectDir"
    SetOutPath "$DetectDir"
    File "..\src\bin\Release\net472\omen-fan-driver.exe"
    File /nonfatal "..\src\bin\Release\net472\Newtonsoft.Json.dll"

    nsExec::ExecToStack '"$DetectDir\omen-fan-driver.exe" --detect-only'
    Pop $DetectExit
    Pop $DetectOutput

    ${If} $DetectExit == "0"
        ${StrStr} $0 $DetectOutput '$\"supported$\":true'
        ${If} $0 != ""
            StrCpy $DetectNeedWarning "0"
        ${EndIf}
    ${EndIf}

    Delete "$DetectDir\omen-fan-driver.exe"
    Delete "$DetectDir\Newtonsoft.Json.dll"
    RMDir "$DetectDir"

    ${If} $DetectNeedWarning != "0"
        MessageBox MB_ICONEXCLAMATION|MB_OK "This machine does not appear to expose supported HP OMEN fan-control WMI. The OMEN Fan Plugin can still be installed for UI preview, mock flows, or debugging, but hardware fan control will not work on non-OMEN or unsupported machines."
    ${EndIf}
FunctionEnd

Function .onInstSuccess
FunctionEnd

Section "Install"
    Call ValidateFanControlRoot
    Call WarnIfOmenUnsupported

    StrCpy $PluginDir "$INSTDIR\plugins\${PLUGIN_ID}"
    SetOutPath "$PluginDir"

    File "..\src\bin\Release\net472\omen-fan-driver.exe"
    File /nonfatal "..\src\bin\Release\net472\Newtonsoft.Json.dll"
    File "..\src\plugin.json"
    CreateDirectory "$PluginDir\ui"
    SetOutPath "$PluginDir\ui"
    File "..\ui\*.js"

    WriteUninstaller "$PluginDir\uninstall-omen-fan.exe"

    SetRegView 64
    WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "DisplayName" "${APP_NAME}"
    WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "DisplayVersion" "${PLUGIN_VERSION}"
    WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "Publisher" "${COMPANY_NAME}"
    WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "InstallLocation" "$PluginDir"
    WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "UninstallString" "$\"$PluginDir\uninstall-omen-fan.exe$\""
    WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "QuietUninstallString" "$\"$PluginDir\uninstall-omen-fan.exe$\" /S"
SectionEnd

Function un.onInit
    SetRegView 64
    ReadRegStr $INSTDIR HKLM "${PLUGIN_UNINSTALL_KEY}" "InstallLocation"
    ${If} $INSTDIR == ""
        StrCpy $INSTDIR "$EXEDIR"
    ${EndIf}
FunctionEnd

Section "Uninstall"
    SetRegView 64

    Delete "$INSTDIR\omen-fan-driver.exe"
    Delete "$INSTDIR\Newtonsoft.Json.dll"
    Delete "$INSTDIR\plugin.json"
    Delete "$INSTDIR\ui\omen-fan.plugin.js"
    Delete "$INSTDIR\ui\omen-core.js"
    Delete "$INSTDIR\ui\omen-style.js"
    Delete "$INSTDIR\ui\omen-components.js"
    Delete "$INSTDIR\ui\omen-views.js"
    Delete "$INSTDIR\ui\omen-app.js"
    Delete "$INSTDIR\ui\index.html"
    Delete "$INSTDIR\uninstall-omen-fan.exe"

    DeleteRegKey HKLM "${PLUGIN_UNINSTALL_KEY}"

    RMDir "$INSTDIR\ui"
    RMDir "$INSTDIR"
SectionEnd
