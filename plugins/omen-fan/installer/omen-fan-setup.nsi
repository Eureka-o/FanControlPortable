Unicode True

!include "MUI2.nsh"
!include "LogicLib.nsh"
!include "x64.nsh"

!ifndef PLUGIN_SOURCE
  !error "PLUGIN_SOURCE must point to the staged omen-fan directory"
!endif
!ifndef OUTPUT_FILE
  !define OUTPUT_FILE "omen-fan-setup.exe"
!endif
!ifndef PLUGIN_VERSION
  !define PLUGIN_VERSION "0.1.0"
!endif

!define PRODUCT_NAME "FanControl OMEN Preview Plugin"
!define MAIN_UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Eureka-o FanControl"
!define PLUGIN_UNINSTALL_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\Eureka-o FanControl OMEN Plugin"

Name "${PRODUCT_NAME}"
OutFile "${OUTPUT_FILE}"
InstallDir "$PROGRAMFILES64\FanControl"
InstallDirRegKey HKLM "${MAIN_UNINSTALL_KEY}" "InstallLocation"
RequestExecutionLevel admin
SetCompressor /SOLID lzma
ShowInstDetails show
ShowUninstDetails show

VIProductVersion "0.1.0.0"
VIAddVersionKey "ProductName" "${PRODUCT_NAME}"
VIAddVersionKey "FileDescription" "HP OMEN plugin preview installer"
VIAddVersionKey "FileVersion" "${PLUGIN_VERSION}"
VIAddVersionKey "ProductVersion" "${PLUGIN_VERSION}"
VIAddVersionKey "CompanyName" "Eureka-o"
VIAddVersionKey "LegalCopyright" "Copyright (c) 2026 Eureka-o"

!insertmacro MUI_PAGE_WELCOME
!insertmacro MUI_PAGE_DIRECTORY
!insertmacro MUI_PAGE_INSTFILES
!insertmacro MUI_PAGE_FINISH
!insertmacro MUI_UNPAGE_CONFIRM
!insertmacro MUI_UNPAGE_INSTFILES
!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"

Function .onInit
  ${IfNot} ${RunningX64}
    MessageBox MB_ICONSTOP|MB_OK "This plugin requires 64-bit Windows."
    Abort
  ${EndIf}
  SetRegView 64
FunctionEnd

Section "OMEN preview plugin" SEC_PLUGIN
  ${IfNot} ${FileExists} "$INSTDIR\FanControl.exe"
    MessageBox MB_ICONSTOP|MB_OK "FanControl.exe was not found in the selected folder. Select the FanControl installation or portable folder."
    Abort
  ${EndIf}

  SetOutPath "$INSTDIR\plugins\omen-fan"
  File /r "${PLUGIN_SOURCE}\*.*"
  WriteUninstaller "$INSTDIR\plugins\omen-fan\uninstall.exe"

  WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "DisplayName" "${PRODUCT_NAME}"
  WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "DisplayVersion" "${PLUGIN_VERSION}"
  WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "Publisher" "Eureka-o"
  WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "InstallLocation" "$INSTDIR\plugins\omen-fan"
  WriteRegStr HKLM "${PLUGIN_UNINSTALL_KEY}" "UninstallString" '"$INSTDIR\plugins\omen-fan\uninstall.exe"'
  WriteRegDWORD HKLM "${PLUGIN_UNINSTALL_KEY}" "NoModify" 1
  WriteRegDWORD HKLM "${PLUGIN_UNINSTALL_KEY}" "NoRepair" 1
SectionEnd

Section "Uninstall"
  SetRegView 64
  DeleteRegKey HKLM "${PLUGIN_UNINSTALL_KEY}"
  RMDir /r "$EXEDIR\backend"
  RMDir /r "$EXEDIR\ui"
  Delete "$EXEDIR\plugin.json"
  Delete "$EXEDIR\THIRD_PARTY_NOTICES.md"
  Delete "$EXEDIR\uninstall.exe"
  RMDir "$EXEDIR"
SectionEnd
