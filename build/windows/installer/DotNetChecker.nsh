# .NET Framework Detection Macros
# Based on .NET Framework Detection NSIS Plugin

!include "LogicLib.nsh"

# Macro to check if .NET Framework version is installed
# Usage: !insertmacro CheckNetFramework VERSION_NUMBER
# VERSION_NUMBER: 472 for .NET 4.7.2, 480 for .NET 4.8, etc.
# Returns: "true" if installed, "false" if not installed
!macro CheckNetFramework VERSION_NUMBER
    Push $0
    Push $1
    Push $2
    
    # Check registry for .NET Framework version
    ClearErrors
    
    # For .NET Framework 4.7.2, we check for release value >= 461808
    ${If} ${VERSION_NUMBER} == 472
        ReadRegDWORD $0 HKLM "SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full" "Release"
        ${If} ${Errors}
            # Try 32-bit registry on 64-bit system
            SetRegView 32
            ReadRegDWORD $0 HKLM "SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full" "Release"
            SetRegView 64
        ${EndIf}
        
        ${If} ${Errors}
            StrCpy $1 "false"
        ${Else}
            # .NET Framework 4.7.2 has release value 461808
            ${If} $0 >= 461808
                StrCpy $1 "true"
            ${Else}
                StrCpy $1 "false"
            ${EndIf}
        ${EndIf}
    ${ElseIf} ${VERSION_NUMBER} == 480
        ReadRegDWORD $0 HKLM "SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full" "Release"
        ${If} ${Errors}
            # Try 32-bit registry on 64-bit system
            SetRegView 32
            ReadRegDWORD $0 HKLM "SOFTWARE\Microsoft\NET Framework Setup\NDP\v4\Full" "Release"
            SetRegView 64
        ${EndIf}
        
        ${If} ${Errors}
            StrCpy $1 "false"
        ${Else}
            # .NET Framework 4.8 has release value 528040
            ${If} $0 >= 528040
                StrCpy $1 "true"
            ${Else}
                StrCpy $1 "false"
            ${EndIf}
        ${EndIf}
    ${Else}
        # Default to false for unsupported version numbers
        StrCpy $1 "false"
    ${EndIf}
    
    Pop $2
    Pop $0
    Push $1
!macroend

# Alternative macro that shows message and downloads .NET Framework if not installed
!macro CheckAndInstallNetFramework VERSION_NUMBER DOWNLOAD_URL
    !insertmacro CheckNetFramework ${VERSION_NUMBER}
    Pop $0
    ${If} $0 == "false"
        MessageBox MB_YESNO|MB_ICONQUESTION ".NET Framework is required. Do you want to download and install it?" IDYES download IDNO skip
        download:
            ExecShell "open" "${DOWNLOAD_URL}"
            MessageBox MB_OK "Please install .NET Framework and then re-run the installer."
            Abort
        skip:
            MessageBox MB_OK ".NET Framework is required. Installation will be terminated."
            Abort
    ${EndIf}
!macroend
