Unicode true

####
## Please note: Template replacements don't work in this file. They are provided with default defines like
## mentioned underneath.
## If the keyword is not defined, "wails_tools.nsh" will populate them with the values from ProjectInfo.
## If they are defined here, "wails_tools.nsh" will not touch them. This allows to use this project.nsi manually
## from outside of Wails for debugging and development of the installer.
##
## For development first make a wails nsis build to populate the "wails_tools.nsh":
## > wails build --target windows/amd64 --nsis
## Then you can call makensis on this file with specifying the path to your binary:
## For a AMD64 only installer:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app.exe
## For a ARM64 only installer:
## > makensis -DARG_WAILS_ARM64_BINARY=..\..\bin\app.exe
## For a installer with both architectures:
## > makensis -DARG_WAILS_AMD64_BINARY=..\..\bin\app-amd64.exe -DARG_WAILS_ARM64_BINARY=..\..\bin\app-arm64.exe
####
## The following information is taken from the ProjectInfo file, but they can be overwritten here.
####
## !define INFO_PROJECTNAME    "MyProject" # Default "{{.Name}}"
## !define INFO_COMPANYNAME    "MyCompany" # Default "{{.Info.CompanyName}}"
## !define INFO_PRODUCTNAME    "MyProduct" # Default "{{.Info.ProductName}}"
## !define INFO_PRODUCTVERSION "1.0.0"     # Default "{{.Info.ProductVersion}}"
## !define INFO_COPYRIGHT      "Copyright" # Default "{{.Info.Copyright}}"
###
## !define PRODUCT_EXECUTABLE  "Application.exe"      # Default "${INFO_PROJECTNAME}.exe"
## !define UNINST_KEY_NAME     "UninstKeyInRegistry"  # Default "${INFO_COMPANYNAME}${INFO_PRODUCTNAME}"
####
## Keep FanControl isolated from other installed fan-control tools.
!define UNINST_KEY_NAME "Eureka-o FanControl"
!define LEGACY_UNINST_KEY_NAME "Eureka-o FanControlPortable"
!define LEGACY_UNINST_KEY "Software\Microsoft\Windows\CurrentVersion\Uninstall\${LEGACY_UNINST_KEY_NAME}"
!define LEGACY_PRODUCTNAME "FanControlPortable"
!define LEGACY_PRODUCT_EXECUTABLE "FanControlPortable.exe"
!define LEGACY_CORE_EXECUTABLE "FanControlPortable Core.exe"
!define LEGACY_BRIDGE_EXECUTABLE "FanControlPortable TempBridge.exe"
####
## !define REQUEST_EXECUTION_LEVEL "admin"            # Default "admin"  see also https://nsis.sourceforge.io/Docs/Chapter4.html
####
## Include the wails tools
####
!include "wails_tools.nsh"

# Include required plugins and libraries
!include "MUI.nsh"
!include "FileFunc.nsh"
!include "WordFunc.nsh"

# Include .NET Framework Detection
!include "DotNetChecker.nsh"
!include "project_strings.nsh"

!macro TryInstallDirCandidate CANDIDATE SOURCE LEGACY
    ${If} "${CANDIDATE}" != ""
        ${If} ${FileExists} "${CANDIDATE}\${PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR "${CANDIDATE}"
            ${If} "${LEGACY}" == "1"
                DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            ${Else}
                DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            ${EndIf}
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\${LEGACY_PRODUCT_EXECUTABLE}"
            StrCpy $INSTDIR "${CANDIDATE}"
            DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            Goto found_installation
        ${EndIf}
        ${If} ${FileExists} "${CANDIDATE}\uninstall.exe"
            StrCpy $INSTDIR "${CANDIDATE}"
            ${If} "${LEGACY}" == "1"
                DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
            ${Else}
                DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
            ${EndIf}
            Goto found_installation
        ${EndIf}
    ${EndIf}
!macroend

# Built-in PawnIO version for install/update decisions.
# You can override this at build time with: -DPAWNIO_BUNDLED_VERSION=x.y.z
!ifndef PAWNIO_BUNDLED_VERSION
!define PAWNIO_BUNDLED_VERSION "2.2.0.0"
!endif

!ifndef CORE_EXECUTABLE_SOURCE
!define CORE_EXECUTABLE_SOURCE "..\..\bin\FanControl Core.exe"
!endif

# The version information for this two must consist of 4 parts
VIProductVersion "${INFO_PRODUCTVERSION}.0"
VIFileVersion    "${INFO_PRODUCTVERSION}.0"

VIAddVersionKey "CompanyName"     "${INFO_COMPANYNAME}"
VIAddVersionKey "FileDescription" "${INFO_PRODUCTNAME} Installer"
VIAddVersionKey "ProductVersion"  "${INFO_PRODUCTVERSION}"
VIAddVersionKey "FileVersion"     "${INFO_PRODUCTVERSION}"
VIAddVersionKey "LegalCopyright"  "${INFO_COPYRIGHT}"
VIAddVersionKey "ProductName"     "${INFO_PRODUCTNAME}"

# Enable HiDPI support. https://nsis.sourceforge.io/Reference/ManifestDPIAware
ManifestDPIAware true

!define MUI_ICON "..\icon.ico"
!define MUI_UNICON "..\icon.ico"
# !define MUI_WELCOMEFINISHPAGE_BITMAP "resources\leftimage.bmp" #Include this to add a bitmap on the left side of the Welcome Page. Must be a size of 164x314
!define MUI_FINISHPAGE_NOAUTOCLOSE # Wait on the INSTFILES page so the user can take a look into the details of the installation steps
!define MUI_FINISHPAGE_RUN "$INSTDIR\${PRODUCT_EXECUTABLE}"
!define MUI_FINISHPAGE_RUN_TEXT "$(THRM_STR_FINISHPAGE_RUN)"
!define MUI_ABORTWARNING # This will warn the user if they exit from the installer.

!define MUI_PAGE_CUSTOMFUNCTION_PRE WelcomePagePre
!insertmacro MUI_PAGE_WELCOME # Welcome to the installer page.
# !insertmacro MUI_PAGE_LICENSE "resources\eula.txt" # Adds a EULA page to the installer
!insertmacro MUI_PAGE_DIRECTORY # In which folder install page.
!insertmacro MUI_PAGE_COMPONENTS # Component selection page
!insertmacro MUI_PAGE_INSTFILES # Installing page.
!insertmacro MUI_PAGE_FINISH # Finished installation page.

!insertmacro MUI_UNPAGE_INSTFILES # Uinstalling page

!insertmacro MUI_LANGUAGE "SimpChinese"
!insertmacro MUI_LANGUAGE "English"
!insertmacro MUI_LANGUAGE "Japanese"

## The following two statements can be used to sign the installer and the uninstaller. The path to the binaries are provided in %1
#!uninstfinalize 'signtool --file "%1"'
#!finalize 'signtool --file "%1"'

Name "${INFO_PRODUCTNAME}"
Caption "$(THRM_STR_CAPTION)"
BrandingText "${INFO_PRODUCTNAME} v${INFO_PRODUCTVERSION}"
OutFile "..\..\bin\${INFO_PROJECTNAME}-${ARCH}-installer.exe" # Name of the installer's file.
InstallDir "$PROGRAMFILES64\${INFO_PRODUCTNAME}" # Default installing folder (single level)
ShowInstDetails show # This will always show the installation details.

Var LegacyRenameNoticeNeeded

Function .onInit
    StrCpy $LegacyRenameNoticeNeeded "0"
   !insertmacro wails.checkArchitecture

   # Check for .NET Framework 4.7.2 or later
   !insertmacro CheckNetFramework 472
   Pop $0
   ${If} $0 == "false"
       MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_REQUIRE_DOTNET)"
       Abort
   ${EndIf}

    # Check for existing installation and set install directory
   Call DetectExistingInstallation
FunctionEnd

Function WelcomePagePre
    ${If} $LegacyRenameNoticeNeeded == "1"
        !insertmacro INSTALLOPTIONS_WRITE "ioSpecial.ini" "Field 2" "Text" "$(THRM_STR_LEGACY_TITLE)"
        !insertmacro INSTALLOPTIONS_WRITE "ioSpecial.ini" "Field 3" "Text" "$(THRM_STR_LEGACY_BODY)"
    ${EndIf}
FunctionEnd

# Function to clean up legacy/duplicate registry keys
Function CleanLegacyRegistryKeys
    # No-op: FanControl must not clean registry keys owned by other apps.
FunctionEnd

# Function to detect existing installation and set install directory
Function DetectExistingInstallation
    DetailPrint "$(THRM_STR_CHECKING_INSTALL)"
    SetRegView 64

    Push $R0
    Push $R1
    Push $R2

    # Only detect FanControl's own install. Do not inspect other fan-control tools.
    # registry keys or folders because the user may run those applications side by side.
    ReadRegStr $R2 HKLM "${UNINST_KEY}" "DisplayVersion"
    ${If} $R2 != ""
        DetailPrint "$(THRM_STR_LOCAL_VERSION) $R2"
    ${Else}
        DetailPrint "$(THRM_STR_NO_LOCAL_VERSION)"
    ${EndIf}

    ReadRegStr $R0 HKLM "${UNINST_KEY}" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "current-key-install-location" "0"

    ReadRegStr $R0 HKLM "${UNINST_KEY}" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "current-key-uninstall-path" "0"
    ${EndIf}

    ReadRegStr $R0 HKLM "${UNINST_KEY}" "DisplayIcon"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "current-key-icon-path" "0"
    ${EndIf}

    ReadRegStr $R0 HKLM "${LEGACY_UNINST_KEY}" "InstallLocation"
    !insertmacro TryInstallDirCandidate "$R0" "legacy-key-install-location" "1"

    ReadRegStr $R0 HKLM "${LEGACY_UNINST_KEY}" "UninstallString"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "legacy-key-uninstall-path" "1"
    ${EndIf}

    ReadRegStr $R0 HKLM "${LEGACY_UNINST_KEY}" "DisplayIcon"
    ${If} $R0 != ""
        Push $R0
        Call TrimQuotes
        Pop $R0
        ${GetParent} $R0 $R1
        !insertmacro TryInstallDirCandidate "$R1" "legacy-key-icon-path" "1"
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES64\${LEGACY_PRODUCTNAME}\${LEGACY_PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${LEGACY_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_LEGACY_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES32\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES32\${INFO_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}

    ${If} ${FileExists} "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}\${PRODUCT_EXECUTABLE}"
        StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_COMPANYNAME}\${INFO_PRODUCTNAME}"
        DetailPrint "$(THRM_STR_FOUND_INSTALL) $INSTDIR"
        Goto found_installation
    ${EndIf}

    StrCpy $INSTDIR "$PROGRAMFILES64\${INFO_PRODUCTNAME}"
    DetailPrint "$(THRM_STR_DEFAULT_DIR) $INSTDIR"
    Goto end_detection

    found_installation:
    DetailPrint "$(THRM_STR_UPGRADE_TARGET) $INSTDIR"

    end_detection:
    Pop $R2
    Pop $R1
    Pop $R0
FunctionEnd

# Function to write current version info to uninstall registry key
Function WriteCurrentVersionInfo
    SetRegView 64
    WriteRegStr HKLM "${UNINST_KEY}" "DisplayVersion" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKLM "${UNINST_KEY}" "Version" "${INFO_PRODUCTVERSION}"
    WriteRegStr HKLM "${UNINST_KEY}" "InstallLocation" "$INSTDIR"
    WriteRegStr HKLM "${UNINST_KEY}" "DisplayName" "${INFO_PRODUCTNAME}"
    WriteRegStr HKLM "${UNINST_KEY}" "Publisher" "${INFO_COMPANYNAME}"
    DetailPrint "$(THRM_STR_WRITE_VERSION) ${INFO_PRODUCTVERSION}"
FunctionEnd

# Helper function to trim quotes from a string
Function TrimQuotes
    Exch $R0 ; Original string
    Push $R1
    Push $R2

    StrCpy $R1 $R0 1 ; First char
    StrCmp $R1 '"' 0 +2
    StrCpy $R0 $R0 "" 1 ; Remove first quote

    StrLen $R2 $R0
    IntOp $R2 $R2 - 1
    StrCpy $R1 $R0 1 $R2 ; Last char
    StrCmp $R1 '"' 0 +2
    StrCpy $R0 $R0 $R2 ; Remove last quote

    Pop $R2
    Pop $R1
    Exch $R0 ; Trimmed string
FunctionEnd

# Function to stop running application instances
Function StopRunningInstances
    DetailPrint "$(THRM_STR_CHECKING_PROCESSES)"

    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "FanControl Core.exe" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_CLOSE_CORE)"
        Sleep 2000
    ${EndIf}

    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "FanControl Core.exe" /T'
    Pop $0
    Pop $1

    ClearErrors
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_CLOSE_APP)"
        Sleep 2000
    ${EndIf}

    # Force kill if still running (ignore errors)
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "FanControl TempBridge.exe" /T'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${LEGACY_CORE_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${LEGACY_PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${LEGACY_BRIDGE_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    DetailPrint "$(THRM_STR_CLEAN_TASKS)"
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "FanControl" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "${LEGACY_PRODUCTNAME}" /f'
    Pop $0
    Pop $1

    # Wait a moment for processes to fully terminate
    DetailPrint "$(THRM_STR_WAIT_TERMINATE)"
    Sleep 2000

    DetailPrint "$(THRM_STR_PROCESS_DONE)"
FunctionEnd

# Function to backup user data before upgrade
Function BackupUserData
    DetailPrint "$(THRM_STR_BACKUP_CONFIG)"

    # Backup configuration files if they exist
    ${If} ${FileExists} "$INSTDIR\config\config.json"
        CopyFiles "$INSTDIR\config\config.json" "$TEMP\fancontrol_config_dir_backup.json"
        DetailPrint "$(THRM_STR_BACKUP_CONFIG_DONE)"
    ${EndIf}

    ${If} ${FileExists} "$INSTDIR\config\hardware-profile.json"
        CopyFiles "$INSTDIR\config\hardware-profile.json" "$TEMP\fancontrol_hardware_profile_backup.json"
        DetailPrint "$(THRM_STR_BACKUP_CONFIG_DONE)"
    ${EndIf}

    ${If} ${FileExists} "$INSTDIR\config.json"
        CopyFiles "$INSTDIR\config.json" "$TEMP\fancontrol_config_backup.json"
        DetailPrint "$(THRM_STR_BACKUP_CONFIG_DONE)"
    ${EndIf}

    # Backup other important user files if needed
    ${If} ${FileExists} "$INSTDIR\settings.ini"
        CopyFiles "$INSTDIR\settings.ini" "$TEMP\fancontrol_settings_backup.ini"
        DetailPrint "$(THRM_STR_BACKUP_SETTINGS_DONE)"
    ${EndIf}
FunctionEnd

# Function to restore user data after upgrade
Function RestoreUserData
    DetailPrint "$(THRM_STR_RESTORE_CONFIG)"

    # Restore configuration files if backup exists
    ${If} ${FileExists} "$TEMP\fancontrol_config_dir_backup.json"
        CreateDirectory "$INSTDIR\config"
        CopyFiles "$TEMP\fancontrol_config_dir_backup.json" "$INSTDIR\config\config.json"
        DetailPrint "$(THRM_STR_RESTORE_CONFIG_DONE)"
    ${EndIf}

    ${If} ${FileExists} "$TEMP\fancontrol_hardware_profile_backup.json"
        CreateDirectory "$INSTDIR\config"
        CopyFiles "$TEMP\fancontrol_hardware_profile_backup.json" "$INSTDIR\config\hardware-profile.json"
        DetailPrint "$(THRM_STR_RESTORE_CONFIG_DONE)"
    ${EndIf}

    ${If} ${FileExists} "$TEMP\fancontrol_config_backup.json"
        CopyFiles "$TEMP\fancontrol_config_backup.json" "$INSTDIR\config.json"
        DetailPrint "$(THRM_STR_RESTORE_CONFIG_DONE)"
    ${EndIf}

    ${If} ${FileExists} "$TEMP\fancontrol_settings_backup.ini"
        CopyFiles "$TEMP\fancontrol_settings_backup.ini" "$INSTDIR\settings.ini"
        Delete "$TEMP\fancontrol_settings_backup.ini"
        DetailPrint "$(THRM_STR_RESTORE_SETTINGS_DONE)"
    ${EndIf}
FunctionEnd

Function CleanupLegacyShortcuts
    DetailPrint "$(THRM_STR_CLEAN_SHORTCUTS)"
    Delete "$SMSTARTUP\FanControl.lnk"
    Delete "$SMPROGRAMS\${LEGACY_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${LEGACY_PRODUCTNAME}.lnk"
    Delete "$SMSTARTUP\${LEGACY_PRODUCTNAME}.lnk"
FunctionEnd

Function un.CleanupLegacyShortcuts
    DetailPrint "$(THRM_STR_CLEAN_SHORTCUTS)"
    Delete "$SMSTARTUP\FanControl.lnk"
    Delete "$SMPROGRAMS\${LEGACY_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${LEGACY_PRODUCTNAME}.lnk"
    Delete "$SMSTARTUP\${LEGACY_PRODUCTNAME}.lnk"
FunctionEnd

Section "$(THRM_STR_SECTION_MAIN)" SEC_MAIN
    SectionIn RO  # Read-only, cannot be deselected
    !insertmacro wails.setShellContext

    Delete "$TEMP\fancontrol_config_dir_backup.json"
    Delete "$TEMP\fancontrol_config_backup.json"
    Delete "$TEMP\fancontrol_hardware_profile_backup.json"
    Delete "$TEMP\fancontrol_settings_backup.ini"

    StrCpy $0 "0"

    # Check if this is an upgrade installation
    ${If} ${FileExists} "$INSTDIR\${PRODUCT_EXECUTABLE}"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\FanControl Core.exe"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\${LEGACY_CORE_EXECUTABLE}"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${ElseIf} ${FileExists} "$INSTDIR\uninstall.exe"
        StrCpy $0 "1"
        DetailPrint "$(THRM_STR_UPGRADING) $INSTDIR"
    ${EndIf}

    ${If} $0 == "1"
        # Backup important files before upgrade
        Call BackupUserData

        # Ensure old instances are completely stopped before upgrading
        Call StopRunningInstances

        # Clean up old files but preserve user data
        DetailPrint "$(THRM_STR_CLEAN_OLD_FILES)"
        Delete "$INSTDIR\${PRODUCT_EXECUTABLE}"
        Delete "$INSTDIR\FanControl Core.exe"
        Delete "$INSTDIR\${LEGACY_PRODUCT_EXECUTABLE}"
        Delete "$INSTDIR\${LEGACY_CORE_EXECUTABLE}"
        Delete "$INSTDIR\${LEGACY_BRIDGE_EXECUTABLE}"
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.log"  # Keep log structure but remove old logs
    ${Else}
        DetailPrint "$(THRM_STR_FRESH_INSTALL) $INSTDIR"

        # Ensure old instances are completely stopped before installing
        Call StopRunningInstances

        # Clean up any leftover files from previous installation
        DetailPrint "$(THRM_STR_CLEAN_LEFTOVERS)"
        RMDir /r "$INSTDIR\bridge"
        Delete "$INSTDIR\logs\*.*"
    ${EndIf}

    !insertmacro wails.webview2runtime

    SetOutPath $INSTDIR

    !insertmacro wails.files

    # Copy core service executable
    DetailPrint "$(THRM_STR_INSTALLING_CORE)"
    File "/oname=FanControl Core.exe" "${CORE_EXECUTABLE_SOURCE}"

    # Copy bridge directory and its contents
    DetailPrint "$(THRM_STR_INSTALLING_BRIDGE)"
    SetOutPath $INSTDIR\bridge
    File /r "..\..\bin\bridge\*.*"

    # Stage built-in themes first; the migration script copies only missing theme directories.
    DetailPrint "$(THRM_STR_INSTALLING_THEMES)"
    RMDir /r "$INSTDIR\.bundled-themes"
    SetOutPath $INSTDIR\.bundled-themes
    File /r "..\..\bin\themes\*.*"
    SetOutPath $INSTDIR\tools
    File /nonfatal "resources\migrate-themes.ps1"
    ${If} ${FileExists} "$INSTDIR\tools\migrate-themes.ps1"
        DetailPrint "$(THRM_STR_MIGRATING_THEMES)"
        nsExec::ExecToStack /TIMEOUT=30000 '"$SYSDIR\WindowsPowerShell\v1.0\powershell.exe" -NoProfile -ExecutionPolicy Bypass -File "$INSTDIR\tools\migrate-themes.ps1" -InstallThemesDir "$INSTDIR\themes" -BundledThemesDir "$INSTDIR\.bundled-themes"'
        Pop $0
        Pop $1
        Delete "$INSTDIR\tools\migrate-themes.ps1"
        RMDir "$INSTDIR\tools"
    ${EndIf}
    RMDir /r "$INSTDIR\.bundled-themes"

    # Return to main install directory
    SetOutPath $INSTDIR

    # Restore user data if this was an upgrade
    Call RestoreUserData

    # Remove existing shortcuts before creating the new ones.
    Call CleanupLegacyShortcuts

    # Create shortcuts
    DetailPrint "$(THRM_STR_CREATING_SHORTCUTS)"
    CreateShortcut "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "" "$INSTDIR\${PRODUCT_EXECUTABLE}" 0
    SetShellVarContext current
    CreateShortCut "$DESKTOP\${INFO_PRODUCTNAME}.lnk" "$INSTDIR\${PRODUCT_EXECUTABLE}" "" "$INSTDIR\${PRODUCT_EXECUTABLE}" 0
    !insertmacro wails.setShellContext

    !insertmacro wails.associateFiles
    !insertmacro wails.associateCustomProtocols

    !insertmacro wails.writeUninstaller
    Call WriteCurrentVersionInfo

    DetailPrint "$(THRM_STR_INSTALL_COMPLETE)"

    ${If} $LegacyRenameNoticeNeeded == "1"
        DetailPrint "$(THRM_STR_UPGRADE_RENAME_DONE)"
    ${ElseIf} ${FileExists} "$TEMP\fancontrol_config_dir_backup.json"
        DetailPrint "$(THRM_STR_UPGRADE_SETTINGS_DONE)"
    ${ElseIf} ${FileExists} "$TEMP\fancontrol_hardware_profile_backup.json"
        DetailPrint "$(THRM_STR_UPGRADE_SETTINGS_DONE)"
    ${ElseIf} ${FileExists} "$TEMP\fancontrol_config_backup.json"
        DetailPrint "$(THRM_STR_UPGRADE_SETTINGS_DONE)"
    ${Else}
        DetailPrint "$(THRM_STR_INSTALL_SUCCESS)"
    ${EndIf}

    ${If} ${FileExists} "$TEMP\fancontrol_config_dir_backup.json"
        Delete "$TEMP\fancontrol_config_dir_backup.json"
    ${EndIf}
    ${If} ${FileExists} "$TEMP\fancontrol_config_backup.json"
        Delete "$TEMP\fancontrol_config_backup.json"
    ${EndIf}
    ${If} ${FileExists} "$TEMP\fancontrol_hardware_profile_backup.json"
        Delete "$TEMP\fancontrol_hardware_profile_backup.json"
    ${EndIf}
SectionEnd

# Auto-start section (selected by default)
Section "$(THRM_STR_SECTION_AUTOSTART)" SEC_AUTOSTART
    DetailPrint "$(THRM_STR_CONFIG_AUTOSTART)"

    # First, remove our existing auto-start entries to ensure clean state.
    DetailPrint "$(THRM_STR_CLEAN_AUTOSTART)"
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "FanControl" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "${LEGACY_PRODUCTNAME}" /f'
    Pop $0
    Pop $1
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "FanControl"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "FanControl"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${LEGACY_PRODUCTNAME}"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "${LEGACY_PRODUCTNAME}"

    # Create new scheduled task for auto-start with admin privileges
    DetailPrint "$(THRM_STR_CREATE_AUTOSTART_TASK)"

    # Use schtasks to create a task that runs at logon with highest privileges
    # The task starts FanControl Core.exe with --autostart after 15 seconds.
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /create /tn "FanControl" /tr "\"$INSTDIR\FanControl Core.exe\" --autostart" /sc onlogon /delay 0000:15 /rl highest /f'
    Pop $0
    Pop $1
    ${If} $0 == 0
        DetailPrint "$(THRM_STR_AUTOSTART_TASK_OK)"
    ${Else}
        DetailPrint "$(THRM_STR_AUTOSTART_TASK_FAIL)"
        # Fallback: use registry auto-start (will trigger UAC on each login)
        WriteRegStr HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "FanControl" '"$INSTDIR\FanControl Core.exe" --autostart'
        DetailPrint "$(THRM_STR_AUTOSTART_REG_OK)"
    ${EndIf}
SectionEnd

# Required PawnIO installer section
Section "$(THRM_STR_SECTION_PAWNIO)" SEC_PAWNIO
    SectionIn RO
    DetailPrint "$(THRM_STR_PREPARE_PAWNIO)"
    Push $5
    Push $6
    Push $7
    Push $8
    Push $9

    SetOutPath "$INSTDIR\drivers\PawnIO"
    Delete "$INSTDIR\drivers\PawnIO\PawnIO_setup.exe"
    File /nonfatal "..\..\bin\PawnIO_setup.exe"
    StrCpy $7 "$INSTDIR\drivers\PawnIO\PawnIO_setup.exe"
    ${IfNot} ${FileExists} "$7"
        MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_PAWNIO_MISSING)"
        Abort
    ${EndIf}

    # Detect any installed PawnIO. The PawnIO setup program refuses in-place
    # installs, so FanControl upgrades must not run it when PawnIO already exists.
    StrCpy $5 ""
    StrCpy $6 ""
    SetRegView 64
    ReadRegStr $5 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "UninstallString"
    ReadRegStr $6 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayVersion"
    ${If} $5 == ""
        ReadRegStr $5 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayName"
    ${EndIf}
    ${If} $5 == ""
        StrCpy $5 $6
    ${EndIf}
    ${If} $5 == ""
        SetRegView 32
        ReadRegStr $5 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "UninstallString"
        ReadRegStr $6 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayVersion"
        ${If} $5 == ""
            ReadRegStr $5 HKLM "SOFTWARE\Microsoft\Windows\CurrentVersion\Uninstall\PawnIO" "DisplayName"
        ${EndIf}
        ${If} $5 == ""
            StrCpy $5 $6
        ${EndIf}
    ${EndIf}
    SetRegView 64
    ${If} $5 == ""
        ReadRegStr $5 HKLM "SYSTEM\CurrentControlSet\Services\PawnIO" "ImagePath"
    ${EndIf}

    # Decide install strategy:
    # $9 = 0 skip, 1 install. Never update over an existing PawnIO install here.
    StrCpy $9 "1"

    ${If} $5 != ""
        ${If} $6 == ""
            StrCpy $6 "$(THRM_STR_PAWNIO_VERSION_UNKNOWN)"
        ${EndIf}
        DetailPrint "$(THRM_STR_PAWNIO_DETECTED) $6, $(THRM_STR_PAWNIO_BUNDLED) ${PAWNIO_BUNDLED_VERSION}"
        DetailPrint "$(THRM_STR_PAWNIO_SKIP)"
        StrCpy $9 "0"
    ${EndIf}

    ${If} $9 == "0"
        DetailPrint "$(THRM_STR_PAWNIO_SKIP_DONE)"
        Goto pawnio_done
    ${EndIf}

    DetailPrint "$(THRM_STR_PAWNIO_SILENT)"
    nsExec::ExecToStack /TIMEOUT=60000 '"$7" -install -silent'
    Pop $0
    Pop $1
    ${If} $0 == "timeout"
        DetailPrint "$(THRM_STR_PAWNIO_TIMEOUT)"
        nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "PawnIO_setup.exe" /T'
        Pop $2
        Pop $3
        ExecWait '"$7" -install' $0
        ${If} $0 == 0
            DetailPrint "$(THRM_STR_PAWNIO_INTERACTIVE_OK)"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_PAWNIO_INTERACTIVE_FAIL)"
            Abort
        ${EndIf}
    ${ElseIf} $0 == 0
        DetailPrint "$(THRM_STR_PAWNIO_SILENT_OK)"
    ${Else}
        DetailPrint "$(THRM_STR_PAWNIO_FALLBACK)"
        ExecWait '"$7" -install' $0
        ${If} $0 == 0
            DetailPrint "$(THRM_STR_PAWNIO_INTERACTIVE_OK)"
        ${Else}
            MessageBox MB_OK|MB_ICONSTOP "$(THRM_STR_PAWNIO_FAIL)"
            Abort
        ${EndIf}
    ${EndIf}

    pawnio_done:
    Pop $9
    Pop $8
    Pop $7
    Pop $6
    Pop $5
SectionEnd

# Section descriptions
!insertmacro MUI_FUNCTION_DESCRIPTION_BEGIN
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_MAIN} "$(THRM_STR_DESC_MAIN)"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_AUTOSTART} "$(THRM_STR_DESC_AUTOSTART)"
    !insertmacro MUI_DESCRIPTION_TEXT ${SEC_PAWNIO} "$(THRM_STR_DESC_PAWNIO)"
!insertmacro MUI_FUNCTION_DESCRIPTION_END

Section "uninstall"
    !insertmacro wails.setShellContext

    # Stop running instances before uninstalling
    DetailPrint "$(THRM_STR_UNINSTALL_STOP)"

    DetailPrint "$(THRM_STR_STOP_CORE)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "FanControl Core.exe" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "FanControl Core.exe" /T'
    Pop $0
    Pop $1

    # Stop main application (ignore errors)
    DetailPrint "$(THRM_STR_STOP_APP)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    Sleep 1000
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    DetailPrint "$(THRM_STR_STOP_BRIDGE)"
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /IM "FanControl TempBridge.exe" /T'
    Pop $0
    Pop $1
    Sleep 500
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "FanControl TempBridge.exe" /T'
    Pop $0
    Pop $1

    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${LEGACY_CORE_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${LEGACY_PRODUCT_EXECUTABLE}" /T'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\taskkill.exe" /F /IM "${LEGACY_BRIDGE_EXECUTABLE}" /T'
    Pop $0
    Pop $1

    # PawnIO owns the shared R0 driver lifecycle; do not stop/delete it from uninstall.

    # Remove auto-start entries
    DetailPrint "$(THRM_STR_REMOVE_AUTOSTART)"

    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "FanControl" /f'
    Pop $0
    Pop $1
    nsExec::ExecToStack '"$SYSDIR\schtasks.exe" /delete /tn "${LEGACY_PRODUCTNAME}" /f'
    Pop $0
    Pop $1

    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "FanControl"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "FanControl"
    DeleteRegValue HKCU "Software\Microsoft\Windows\CurrentVersion\Run" "${LEGACY_PRODUCTNAME}"
    DeleteRegValue HKLM "Software\Microsoft\Windows\CurrentVersion\Run" "${LEGACY_PRODUCTNAME}"

    Delete "$SMSTARTUP\FanControl.lnk"
    Delete "$SMSTARTUP\${LEGACY_PRODUCTNAME}.lnk"

    # Wait for processes to fully terminate
    Sleep 2000

    # Remove application data directories
    DetailPrint "$(THRM_STR_REMOVE_APPDATA)"
    RMDir /r "$AppData\${PRODUCT_EXECUTABLE}" # Remove the WebView2 DataPath
    RMDir /r "$APPDATA\FanControl"
    RMDir /r "$LOCALAPPDATA\FanControl"
    RMDir /r "$TEMP\FanControl"
    RMDir /r "$APPDATA\${LEGACY_PRODUCTNAME}"
    RMDir /r "$LOCALAPPDATA\${LEGACY_PRODUCTNAME}"
    RMDir /r "$TEMP\${LEGACY_PRODUCTNAME}"

    # Remove installation directory and all contents
    DetailPrint "$(THRM_STR_REMOVE_INSTALL_FILES)"

    # Remove bridge directory.
    DetailPrint "$(THRM_STR_REMOVE_BRIDGE)"
    RMDir /r "$INSTDIR\bridge"

    # Remove logs directory
    DetailPrint "$(THRM_STR_REMOVE_LOGS)"
    RMDir /r "$INSTDIR\logs"

    # Remove the install root only when no user-managed files remain.
    DetailPrint "$(THRM_STR_REMOVE_DIR)"
    RMDir $INSTDIR

    # Remove shortcuts
    DetailPrint "$(THRM_STR_REMOVE_SHORTCUTS)"
    Call un.CleanupLegacyShortcuts
    Delete "$SMPROGRAMS\${INFO_PRODUCTNAME}.lnk"
    Delete "$DESKTOP\${INFO_PRODUCTNAME}.lnk"
    Delete "$SMSTARTUP\FanControl.lnk"
    Delete "$SMSTARTUP\${LEGACY_PRODUCTNAME}.lnk"

    !insertmacro wails.unassociateFiles
    !insertmacro wails.unassociateCustomProtocols

    !insertmacro wails.deleteUninstaller

    DetailPrint "$(THRM_STR_UNINSTALL_COMPLETE)"

    # Optional: Ask user if they want to remove configuration files
    MessageBox MB_YESNO|MB_ICONQUESTION "$(THRM_STR_UNINSTALL_REMOVE_CONFIG)" IDNO skip_config
    RMDir /r "$APPDATA\FanControl"
    RMDir /r "$LOCALAPPDATA\FanControl"
    RMDir /r "$APPDATA\${LEGACY_PRODUCTNAME}"
    RMDir /r "$LOCALAPPDATA\${LEGACY_PRODUCTNAME}"
    skip_config:
SectionEnd
