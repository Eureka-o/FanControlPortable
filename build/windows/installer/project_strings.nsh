!ifndef THRM_INSTALLER_STRINGS_INCLUDED
!define THRM_INSTALLER_STRINGS_INCLUDED

!ifndef LANG_SIMPCHINESE
!define LANG_SIMPCHINESE 2052
!endif

!ifndef LANG_ENGLISH
!define LANG_ENGLISH 1033
!endif

!ifndef LANG_JAPANESE
!define LANG_JAPANESE 1041
!endif

LangString THRM_STR_CAPTION ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME} 安装程序 v${INFO_PRODUCTVERSION}"
LangString THRM_STR_CAPTION ${LANG_ENGLISH} "${INFO_PRODUCTNAME} Installer v${INFO_PRODUCTVERSION}"
LangString THRM_STR_CAPTION ${LANG_JAPANESE} "${INFO_PRODUCTNAME} インストーラー v${INFO_PRODUCTVERSION}"

LangString THRM_STR_FINISHPAGE_RUN ${LANG_SIMPCHINESE} "安装完成后立即启动 ${INFO_PRODUCTNAME}"
LangString THRM_STR_FINISHPAGE_RUN ${LANG_ENGLISH} "Launch ${INFO_PRODUCTNAME} after installation"
LangString THRM_STR_FINISHPAGE_RUN ${LANG_JAPANESE} "インストール完了後に ${INFO_PRODUCTNAME} を起動する"

LangString THRM_STR_REQUIRE_DOTNET ${LANG_SIMPCHINESE} "需要 .NET Framework 4.7.2 或更高版本。$\n$\n请先安装 .NET Framework 4.7.2。"
LangString THRM_STR_REQUIRE_DOTNET ${LANG_ENGLISH} ".NET Framework 4.7.2 or later is required.$\n$\nPlease install .NET Framework 4.7.2 first."
LangString THRM_STR_REQUIRE_DOTNET ${LANG_JAPANESE} ".NET Framework 4.7.2 以降が必要です。$\n$\n先に .NET Framework 4.7.2 をインストールしてください。"

LangString THRM_STR_LEGACY_TITLE ${LANG_SIMPCHINESE} "安装前提示：检测到旧版 ${INFO_PRODUCTNAME}"
LangString THRM_STR_LEGACY_TITLE ${LANG_ENGLISH} "Before installation: an older ${INFO_PRODUCTNAME} installation was detected"
LangString THRM_STR_LEGACY_TITLE ${LANG_JAPANESE} "インストール前のお知らせ: 古い ${INFO_PRODUCTNAME} を検出しました"

LangString THRM_STR_LEGACY_BODY ${LANG_SIMPCHINESE} "检测到本机已有 ${INFO_PRODUCTNAME} 安装。$\r$\n$\r$\n本次安装将作为同一软件的版本更新处理：$\r$\n1. 自动保留现有配置和用户数据；$\r$\n2. 默认继续使用当前安装目录；$\r$\n3. 只会处理 ${INFO_PRODUCTNAME} 自身的文件、快捷方式和自启动项。"
LangString THRM_STR_LEGACY_BODY ${LANG_ENGLISH} "An existing ${INFO_PRODUCTNAME} installation was detected.$\r$\n$\r$\nThis setup will update the same application:$\r$\n1. Keep existing configuration and user data automatically;$\r$\n2. Continue using the current install directory by default;$\r$\n3. Only touch ${INFO_PRODUCTNAME} files, shortcuts, and startup entries."
LangString THRM_STR_LEGACY_BODY ${LANG_JAPANESE} "既存の ${INFO_PRODUCTNAME} インストールを検出しました。$\r$\n$\r$\nこのセットアップは同じアプリケーションの更新として処理します:$\r$\n1. 既存の設定とユーザーデータを自動的に保持します。$\r$\n2. 既定では現在のインストール先を引き続き使用します。$\r$\n3. ${INFO_PRODUCTNAME} のファイル、ショートカット、自動起動項目のみを処理します。"

LangString THRM_STR_SECTION_MAIN ${LANG_SIMPCHINESE} "主程序 (必需)"
LangString THRM_STR_SECTION_MAIN ${LANG_ENGLISH} "Main application (required)"
LangString THRM_STR_SECTION_MAIN ${LANG_JAPANESE} "メインアプリケーション (必須)"

LangString THRM_STR_SECTION_AUTOSTART ${LANG_SIMPCHINESE} "开机自启动"
LangString THRM_STR_SECTION_AUTOSTART ${LANG_ENGLISH} "Start with Windows"
LangString THRM_STR_SECTION_AUTOSTART ${LANG_JAPANESE} "Windows 起動時に開始"

LangString THRM_STR_SECTION_PAWNIO ${LANG_SIMPCHINESE} "安装 PawnIO (必需)"
LangString THRM_STR_SECTION_PAWNIO ${LANG_ENGLISH} "Install PawnIO (required)"
LangString THRM_STR_SECTION_PAWNIO ${LANG_JAPANESE} "PawnIO をインストール (必須)"

LangString THRM_STR_DESC_MAIN ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME} 主程序和核心服务文件。"
LangString THRM_STR_DESC_MAIN ${LANG_ENGLISH} "${INFO_PRODUCTNAME} main application and core service files."
LangString THRM_STR_DESC_MAIN ${LANG_JAPANESE} "${INFO_PRODUCTNAME} 本体とコアサービスのファイルです。"

LangString THRM_STR_DESC_AUTOSTART ${LANG_SIMPCHINESE} "系统启动时自动运行 FanControlPortable Core。推荐开启。"
LangString THRM_STR_DESC_AUTOSTART ${LANG_ENGLISH} "Start FanControlPortable Core automatically when Windows starts. Recommended."
LangString THRM_STR_DESC_AUTOSTART ${LANG_JAPANESE} "Windows 起動時に FanControlPortable Core を自動実行します。推奨です。"

LangString THRM_STR_DESC_PAWNIO ${LANG_SIMPCHINESE} "安装 PawnIO 驱动，PawnIO 将用于获取硬件相关信息。"
LangString THRM_STR_DESC_PAWNIO ${LANG_ENGLISH} "Install the PawnIO driver used to read hardware-related information."
LangString THRM_STR_DESC_PAWNIO ${LANG_JAPANESE} "ハードウェア情報の取得に使用する PawnIO ドライバーをインストールします。"

LangString THRM_STR_PAWNIO_MISSING ${LANG_SIMPCHINESE} "未找到 PawnIO_setup.exe（build\\bin）。请先执行 build_bridge.bat 下载后再打包安装器。"
LangString THRM_STR_PAWNIO_MISSING ${LANG_ENGLISH} "PawnIO_setup.exe was not found in build\\bin. Run build_bridge.bat to download it before packaging the installer."
LangString THRM_STR_PAWNIO_MISSING ${LANG_JAPANESE} "build\\bin に PawnIO_setup.exe が見つかりません。先に build_bridge.bat を実行してダウンロードしてからインストーラーを作成してください。"

LangString THRM_STR_PAWNIO_INTERACTIVE_FAIL ${LANG_SIMPCHINESE} "PawnIO 交互安装/更新失败（返回码: $0）。$\n$\n常见原因：驱动服务被系统标记删除（错误 1072）。$\n请先重启系统后重新运行安装程序。"
LangString THRM_STR_PAWNIO_INTERACTIVE_FAIL ${LANG_ENGLISH} "PawnIO interactive install/update failed (exit code: $0).$\n$\nCommon cause: the driver service is marked for deletion (error 1072).$\nPlease reboot and run the installer again."
LangString THRM_STR_PAWNIO_INTERACTIVE_FAIL ${LANG_JAPANESE} "PawnIO の対話式インストール/更新に失敗しました (終了コード: $0)。$\n$\n一般的な原因: ドライバーサービスが削除待ちとしてマークされています (エラー 1072)。$\nシステムを再起動してから、もう一度インストーラーを実行してください。"

LangString THRM_STR_PAWNIO_FAIL ${LANG_SIMPCHINESE} "PawnIO 安装/更新失败（返回码: $0）。$\n$\n常见原因：驱动服务被系统标记删除（错误 1072）。$\n请先重启系统后重新运行安装程序。"
LangString THRM_STR_PAWNIO_FAIL ${LANG_ENGLISH} "PawnIO install/update failed (exit code: $0).$\n$\nCommon cause: the driver service is marked for deletion (error 1072).$\nPlease reboot and run the installer again."
LangString THRM_STR_PAWNIO_FAIL ${LANG_JAPANESE} "PawnIO のインストール/更新に失敗しました (終了コード: $0)。$\n$\n一般的な原因: ドライバーサービスが削除待ちとしてマークされています (エラー 1072)。$\nシステムを再起動してから、もう一度インストーラーを実行してください。"

LangString THRM_STR_UNINSTALL_REMOVE_CONFIG ${LANG_SIMPCHINESE} "是否删除所有配置文件和日志？"
LangString THRM_STR_UNINSTALL_REMOVE_CONFIG ${LANG_ENGLISH} "Remove all configuration files and logs?"
LangString THRM_STR_UNINSTALL_REMOVE_CONFIG ${LANG_JAPANESE} "すべての設定ファイルとログを削除しますか？"

LangString THRM_STR_FOUND_INSTALL ${LANG_SIMPCHINESE} "发现已有安装:"
LangString THRM_STR_FOUND_INSTALL ${LANG_ENGLISH} "Found existing installation:"
LangString THRM_STR_FOUND_INSTALL ${LANG_JAPANESE} "既存のインストールを検出しました:"

LangString THRM_STR_FOUND_LEGACY_INSTALL ${LANG_SIMPCHINESE} "发现已有安装:"
LangString THRM_STR_FOUND_LEGACY_INSTALL ${LANG_ENGLISH} "Found existing installation:"
LangString THRM_STR_FOUND_LEGACY_INSTALL ${LANG_JAPANESE} "既存のインストールを検出しました:"

LangString THRM_STR_CLEANING_LEGACY_REG ${LANG_SIMPCHINESE} "正在清理 ${INFO_PRODUCTNAME} 历史注册表项..."
LangString THRM_STR_CLEANING_LEGACY_REG ${LANG_ENGLISH} "Cleaning ${INFO_PRODUCTNAME} registry keys..."
LangString THRM_STR_CLEANING_LEGACY_REG ${LANG_JAPANESE} "${INFO_PRODUCTNAME} のレジストリキーを整理しています..."

LangString THRM_STR_FOUND_REGKEY ${LANG_SIMPCHINESE} "发现注册表项:"
LangString THRM_STR_FOUND_REGKEY ${LANG_ENGLISH} "Found registry key:"
LangString THRM_STR_FOUND_REGKEY ${LANG_JAPANESE} "レジストリキーを検出しました:"

LangString THRM_STR_REMOVED_REGKEY ${LANG_SIMPCHINESE} "已删除注册表项"
LangString THRM_STR_REMOVED_REGKEY ${LANG_ENGLISH} "Removed registry key"
LangString THRM_STR_REMOVED_REGKEY ${LANG_JAPANESE} "レジストリキーを削除しました"

LangString THRM_STR_CHECKING_INSTALL ${LANG_SIMPCHINESE} "正在检查已有安装..."
LangString THRM_STR_CHECKING_INSTALL ${LANG_ENGLISH} "Checking for existing installation..."
LangString THRM_STR_CHECKING_INSTALL ${LANG_JAPANESE} "既存のインストールを確認しています..."

LangString THRM_STR_LOCAL_VERSION ${LANG_SIMPCHINESE} "本地已安装版本:"
LangString THRM_STR_LOCAL_VERSION ${LANG_ENGLISH} "Locally installed version:"
LangString THRM_STR_LOCAL_VERSION ${LANG_JAPANESE} "ローカルにインストール済みのバージョン:"

LangString THRM_STR_NO_LOCAL_VERSION ${LANG_SIMPCHINESE} "本地未检测到已安装版本信息"
LangString THRM_STR_NO_LOCAL_VERSION ${LANG_ENGLISH} "No local installed version information was detected"
LangString THRM_STR_NO_LOCAL_VERSION ${LANG_JAPANESE} "ローカルにインストール済みバージョン情報は見つかりませんでした"

LangString THRM_STR_DEFAULT_DIR ${LANG_SIMPCHINESE} "未发现已有安装，使用默认目录:"
LangString THRM_STR_DEFAULT_DIR ${LANG_ENGLISH} "No existing installation was found. Using default directory:"
LangString THRM_STR_DEFAULT_DIR ${LANG_JAPANESE} "既存のインストールは見つかりませんでした。既定のディレクトリを使用します:"

LangString THRM_STR_UPGRADE_TARGET ${LANG_SIMPCHINESE} "检测到已有安装，将执行升级到:"
LangString THRM_STR_UPGRADE_TARGET ${LANG_ENGLISH} "Existing installation detected. Upgrading to:"
LangString THRM_STR_UPGRADE_TARGET ${LANG_JAPANESE} "既存のインストールを検出しました。次の場所へアップグレードします:"

LangString THRM_STR_WRITE_VERSION ${LANG_SIMPCHINESE} "已写入版本信息:"
LangString THRM_STR_WRITE_VERSION ${LANG_ENGLISH} "Version information written:"
LangString THRM_STR_WRITE_VERSION ${LANG_JAPANESE} "バージョン情報を書き込みました:"

LangString THRM_STR_CHECKING_PROCESSES ${LANG_SIMPCHINESE} "正在检查运行中的进程..."
LangString THRM_STR_CHECKING_PROCESSES ${LANG_ENGLISH} "Checking running processes..."
LangString THRM_STR_CHECKING_PROCESSES ${LANG_JAPANESE} "実行中のプロセスを確認しています..."

LangString THRM_STR_CLOSE_CORE ${LANG_SIMPCHINESE} "已请求关闭 FanControlPortable Core.exe..."
LangString THRM_STR_CLOSE_CORE ${LANG_ENGLISH} "Requested FanControlPortable Core.exe to close..."
LangString THRM_STR_CLOSE_CORE ${LANG_JAPANESE} "FanControlPortable Core.exe の終了を要求しました..."

LangString THRM_STR_CLOSE_LEGACY_CORE ${LANG_SIMPCHINESE} "已请求关闭历史核心服务..."
LangString THRM_STR_CLOSE_LEGACY_CORE ${LANG_ENGLISH} "Requested previous core service to close..."
LangString THRM_STR_CLOSE_LEGACY_CORE ${LANG_JAPANESE} "以前のコアサービスの終了を要求しました..."

LangString THRM_STR_CLOSE_SPACESTATION ${LANG_SIMPCHINESE} "已请求关闭冲突服务..."
LangString THRM_STR_CLOSE_SPACESTATION ${LANG_ENGLISH} "Requested conflicting service to close..."
LangString THRM_STR_CLOSE_SPACESTATION ${LANG_JAPANESE} "競合サービスの終了を要求しました..."

LangString THRM_STR_CLOSE_APP ${LANG_SIMPCHINESE} "已请求关闭 ${PRODUCT_EXECUTABLE}..."
LangString THRM_STR_CLOSE_APP ${LANG_ENGLISH} "Requested ${PRODUCT_EXECUTABLE} to close..."
LangString THRM_STR_CLOSE_APP ${LANG_JAPANESE} "${PRODUCT_EXECUTABLE} の終了を要求しました..."

LangString THRM_STR_CLEAN_TASKS ${LANG_SIMPCHINESE} "正在清理计划任务..."
LangString THRM_STR_CLEAN_TASKS ${LANG_ENGLISH} "Cleaning scheduled tasks..."
LangString THRM_STR_CLEAN_TASKS ${LANG_JAPANESE} "スケジュールタスクを整理しています..."

LangString THRM_STR_WAIT_TERMINATE ${LANG_SIMPCHINESE} "等待进程完全终止..."
LangString THRM_STR_WAIT_TERMINATE ${LANG_ENGLISH} "Waiting for processes to fully terminate..."
LangString THRM_STR_WAIT_TERMINATE ${LANG_JAPANESE} "プロセスが完全に終了するのを待っています..."

LangString THRM_STR_PROCESS_DONE ${LANG_SIMPCHINESE} "进程清理完成"
LangString THRM_STR_PROCESS_DONE ${LANG_ENGLISH} "Process cleanup complete"
LangString THRM_STR_PROCESS_DONE ${LANG_JAPANESE} "プロセスの整理が完了しました"

LangString THRM_STR_BACKUP_CONFIG ${LANG_SIMPCHINESE} "正在备份用户配置..."
LangString THRM_STR_BACKUP_CONFIG ${LANG_ENGLISH} "Backing up user configuration..."
LangString THRM_STR_BACKUP_CONFIG ${LANG_JAPANESE} "ユーザー設定をバックアップしています..."

LangString THRM_STR_BACKUP_CONFIG_DONE ${LANG_SIMPCHINESE} "配置文件已备份"
LangString THRM_STR_BACKUP_CONFIG_DONE ${LANG_ENGLISH} "Configuration file backed up"
LangString THRM_STR_BACKUP_CONFIG_DONE ${LANG_JAPANESE} "設定ファイルをバックアップしました"

LangString THRM_STR_BACKUP_SETTINGS_DONE ${LANG_SIMPCHINESE} "设置文件已备份"
LangString THRM_STR_BACKUP_SETTINGS_DONE ${LANG_ENGLISH} "Settings file backed up"
LangString THRM_STR_BACKUP_SETTINGS_DONE ${LANG_JAPANESE} "設定ファイルをバックアップしました"

LangString THRM_STR_RESTORE_CONFIG ${LANG_SIMPCHINESE} "正在恢复用户配置..."
LangString THRM_STR_RESTORE_CONFIG ${LANG_ENGLISH} "Restoring user configuration..."
LangString THRM_STR_RESTORE_CONFIG ${LANG_JAPANESE} "ユーザー設定を復元しています..."

LangString THRM_STR_RESTORE_CONFIG_DONE ${LANG_SIMPCHINESE} "配置文件已恢复"
LangString THRM_STR_RESTORE_CONFIG_DONE ${LANG_ENGLISH} "Configuration file restored"
LangString THRM_STR_RESTORE_CONFIG_DONE ${LANG_JAPANESE} "設定ファイルを復元しました"

LangString THRM_STR_RESTORE_SETTINGS_DONE ${LANG_SIMPCHINESE} "设置文件已恢复"
LangString THRM_STR_RESTORE_SETTINGS_DONE ${LANG_ENGLISH} "Settings file restored"
LangString THRM_STR_RESTORE_SETTINGS_DONE ${LANG_JAPANESE} "設定ファイルを復元しました"

LangString THRM_STR_CLEAN_SHORTCUTS ${LANG_SIMPCHINESE} "正在清理快捷方式..."
LangString THRM_STR_CLEAN_SHORTCUTS ${LANG_ENGLISH} "Cleaning shortcuts..."
LangString THRM_STR_CLEAN_SHORTCUTS ${LANG_JAPANESE} "ショートカットを整理しています..."

LangString THRM_STR_UPGRADING ${LANG_SIMPCHINESE} "正在升级:"
LangString THRM_STR_UPGRADING ${LANG_ENGLISH} "Upgrading:"
LangString THRM_STR_UPGRADING ${LANG_JAPANESE} "アップグレード中:"

LangString THRM_STR_CLEAN_OLD_FILES ${LANG_SIMPCHINESE} "正在清理历史版本文件..."
LangString THRM_STR_CLEAN_OLD_FILES ${LANG_ENGLISH} "Cleaning previous version files..."
LangString THRM_STR_CLEAN_OLD_FILES ${LANG_JAPANESE} "以前のバージョンのファイルを整理しています..."

LangString THRM_STR_FRESH_INSTALL ${LANG_SIMPCHINESE} "全新安装:"
LangString THRM_STR_FRESH_INSTALL ${LANG_ENGLISH} "Fresh installation:"
LangString THRM_STR_FRESH_INSTALL ${LANG_JAPANESE} "新規インストール:"

LangString THRM_STR_CLEAN_LEFTOVERS ${LANG_SIMPCHINESE} "正在清理残留文件..."
LangString THRM_STR_CLEAN_LEFTOVERS ${LANG_ENGLISH} "Cleaning leftover files..."
LangString THRM_STR_CLEAN_LEFTOVERS ${LANG_JAPANESE} "残留ファイルを整理しています..."

LangString THRM_STR_INSTALLING_CORE ${LANG_SIMPCHINESE} "正在安装核心服务..."
LangString THRM_STR_INSTALLING_CORE ${LANG_ENGLISH} "Installing core service..."
LangString THRM_STR_INSTALLING_CORE ${LANG_JAPANESE} "コアサービスをインストールしています..."

LangString THRM_STR_INSTALLING_BRIDGE ${LANG_SIMPCHINESE} "正在安装桥接组件..."
LangString THRM_STR_INSTALLING_BRIDGE ${LANG_ENGLISH} "Installing bridge components..."
LangString THRM_STR_INSTALLING_BRIDGE ${LANG_JAPANESE} "ブリッジコンポーネントをインストールしています..."

LangString THRM_STR_CREATING_SHORTCUTS ${LANG_SIMPCHINESE} "正在创建快捷方式..."
LangString THRM_STR_CREATING_SHORTCUTS ${LANG_ENGLISH} "Creating shortcuts..."
LangString THRM_STR_CREATING_SHORTCUTS ${LANG_JAPANESE} "ショートカットを作成しています..."

LangString THRM_STR_INSTALL_COMPLETE ${LANG_SIMPCHINESE} "安装完成"
LangString THRM_STR_INSTALL_COMPLETE ${LANG_ENGLISH} "Installation complete"
LangString THRM_STR_INSTALL_COMPLETE ${LANG_JAPANESE} "インストールが完了しました"

LangString THRM_STR_UPGRADE_RENAME_DONE ${LANG_SIMPCHINESE} "已完成 ${INFO_PRODUCTNAME} 更新安装。"
LangString THRM_STR_UPGRADE_RENAME_DONE ${LANG_ENGLISH} "${INFO_PRODUCTNAME} update installation completed."
LangString THRM_STR_UPGRADE_RENAME_DONE ${LANG_JAPANESE} "${INFO_PRODUCTNAME} の更新インストールが完了しました。"

LangString THRM_STR_UPGRADE_SETTINGS_DONE ${LANG_SIMPCHINESE} "已完成升级，原有设置已保留。"
LangString THRM_STR_UPGRADE_SETTINGS_DONE ${LANG_ENGLISH} "Upgrade completed and existing settings were preserved."
LangString THRM_STR_UPGRADE_SETTINGS_DONE ${LANG_JAPANESE} "アップグレードが完了し、既存の設定を保持しました。"

LangString THRM_STR_INSTALL_SUCCESS ${LANG_SIMPCHINESE} "${INFO_PRODUCTNAME} 安装成功。"
LangString THRM_STR_INSTALL_SUCCESS ${LANG_ENGLISH} "${INFO_PRODUCTNAME} was installed successfully."
LangString THRM_STR_INSTALL_SUCCESS ${LANG_JAPANESE} "${INFO_PRODUCTNAME} のインストールに成功しました。"

LangString THRM_STR_CONFIG_AUTOSTART ${LANG_SIMPCHINESE} "正在配置开机自启动..."
LangString THRM_STR_CONFIG_AUTOSTART ${LANG_ENGLISH} "Configuring auto start..."
LangString THRM_STR_CONFIG_AUTOSTART ${LANG_JAPANESE} "自動起動を設定しています..."

LangString THRM_STR_CLEAN_AUTOSTART ${LANG_SIMPCHINESE} "正在清理现有自启动项..."
LangString THRM_STR_CLEAN_AUTOSTART ${LANG_ENGLISH} "Cleaning existing auto-start entries..."
LangString THRM_STR_CLEAN_AUTOSTART ${LANG_JAPANESE} "既存の自動起動設定を整理しています..."

LangString THRM_STR_CREATE_AUTOSTART_TASK ${LANG_SIMPCHINESE} "正在创建自启动计划任务..."
LangString THRM_STR_CREATE_AUTOSTART_TASK ${LANG_ENGLISH} "Creating auto-start scheduled task..."
LangString THRM_STR_CREATE_AUTOSTART_TASK ${LANG_JAPANESE} "自動起動用のスケジュールタスクを作成しています..."

LangString THRM_STR_AUTOSTART_TASK_OK ${LANG_SIMPCHINESE} "开机自启动配置成功（计划任务）"
LangString THRM_STR_AUTOSTART_TASK_OK ${LANG_ENGLISH} "Auto start configured successfully (scheduled task)"
LangString THRM_STR_AUTOSTART_TASK_OK ${LANG_JAPANESE} "自動起動の設定に成功しました (スケジュールタスク)"

LangString THRM_STR_AUTOSTART_TASK_FAIL ${LANG_SIMPCHINESE} "计划任务创建失败，使用注册表方式..."
LangString THRM_STR_AUTOSTART_TASK_FAIL ${LANG_ENGLISH} "Failed to create the scheduled task. Falling back to registry auto start..."
LangString THRM_STR_AUTOSTART_TASK_FAIL ${LANG_JAPANESE} "スケジュールタスクの作成に失敗したため、レジストリ方式に切り替えます..."

LangString THRM_STR_AUTOSTART_REG_OK ${LANG_SIMPCHINESE} "开机自启动配置成功（注册表）"
LangString THRM_STR_AUTOSTART_REG_OK ${LANG_ENGLISH} "Auto start configured successfully (registry)"
LangString THRM_STR_AUTOSTART_REG_OK ${LANG_JAPANESE} "自動起動の設定に成功しました (レジストリ)"

LangString THRM_STR_PREPARE_PAWNIO ${LANG_SIMPCHINESE} "正在准备安装 PawnIO..."
LangString THRM_STR_PREPARE_PAWNIO ${LANG_ENGLISH} "Preparing to install PawnIO..."
LangString THRM_STR_PREPARE_PAWNIO ${LANG_JAPANESE} "PawnIO のインストール準備をしています..."

LangString THRM_STR_PAWNIO_DETECTED ${LANG_SIMPCHINESE} "检测到已安装 PawnIO，版本:"
LangString THRM_STR_PAWNIO_DETECTED ${LANG_ENGLISH} "Detected installed PawnIO version:"
LangString THRM_STR_PAWNIO_DETECTED ${LANG_JAPANESE} "インストール済みの PawnIO バージョンを検出しました:"

LangString THRM_STR_PAWNIO_BUNDLED ${LANG_SIMPCHINESE} "内置版本:"
LangString THRM_STR_PAWNIO_BUNDLED ${LANG_ENGLISH} "Bundled version:"
LangString THRM_STR_PAWNIO_BUNDLED ${LANG_JAPANESE} "同梱バージョン:"

LangString THRM_STR_PAWNIO_UPDATE ${LANG_SIMPCHINESE} "检测到 PawnIO 旧版本，将直接尝试静默更新；不会先卸载共享驱动。"
LangString THRM_STR_PAWNIO_UPDATE ${LANG_ENGLISH} "An older PawnIO version was detected. A silent update will be attempted without uninstalling the shared driver first."
LangString THRM_STR_PAWNIO_UPDATE ${LANG_JAPANESE} "古い PawnIO バージョンを検出しました。共有ドライバーを先に削除せず、そのままサイレント更新を試みます。"

LangString THRM_STR_PAWNIO_SKIP ${LANG_SIMPCHINESE} "PawnIO 已安装且版本满足要求，跳过驱动安装。"
LangString THRM_STR_PAWNIO_SKIP ${LANG_ENGLISH} "PawnIO is already installed and satisfies the required version. Skipping driver installation."
LangString THRM_STR_PAWNIO_SKIP ${LANG_JAPANESE} "PawnIO は既にインストールされており、必要なバージョンを満たしています。ドライバーのインストールをスキップします。"

LangString THRM_STR_PAWNIO_SKIP_DONE ${LANG_SIMPCHINESE} "跳过 PawnIO 处理。"
LangString THRM_STR_PAWNIO_SKIP_DONE ${LANG_ENGLISH} "Skipping PawnIO processing."
LangString THRM_STR_PAWNIO_SKIP_DONE ${LANG_JAPANESE} "PawnIO の処理をスキップします。"

LangString THRM_STR_PAWNIO_SILENT ${LANG_SIMPCHINESE} "正在静默安装/更新 PawnIO（最多等待 60 秒）..."
LangString THRM_STR_PAWNIO_SILENT ${LANG_ENGLISH} "Installing/updating PawnIO silently (waiting up to 60 seconds)..."
LangString THRM_STR_PAWNIO_SILENT ${LANG_JAPANESE} "PawnIO をサイレントでインストール/更新しています (最大 60 秒待機)..."

LangString THRM_STR_PAWNIO_TIMEOUT ${LANG_SIMPCHINESE} "PawnIO 静默安装/更新 60 秒未响应，回退到交互安装..."
LangString THRM_STR_PAWNIO_TIMEOUT ${LANG_ENGLISH} "PawnIO silent install/update did not respond within 60 seconds. Falling back to interactive installation..."
LangString THRM_STR_PAWNIO_TIMEOUT ${LANG_JAPANESE} "PawnIO のサイレントインストール/更新が 60 秒以内に応答しませんでした。対話式インストールに切り替えます..."

LangString THRM_STR_PAWNIO_INTERACTIVE_OK ${LANG_SIMPCHINESE} "PawnIO 安装/更新完成（交互）"
LangString THRM_STR_PAWNIO_INTERACTIVE_OK ${LANG_ENGLISH} "PawnIO install/update completed (interactive)"
LangString THRM_STR_PAWNIO_INTERACTIVE_OK ${LANG_JAPANESE} "PawnIO のインストール/更新が完了しました (対話式)"

LangString THRM_STR_PAWNIO_SILENT_OK ${LANG_SIMPCHINESE} "PawnIO 安装/更新完成（静默）"
LangString THRM_STR_PAWNIO_SILENT_OK ${LANG_ENGLISH} "PawnIO install/update completed (silent)"
LangString THRM_STR_PAWNIO_SILENT_OK ${LANG_JAPANESE} "PawnIO のインストール/更新が完了しました (サイレント)"

LangString THRM_STR_PAWNIO_FALLBACK ${LANG_SIMPCHINESE} "PawnIO 静默安装/更新失败，改为交互安装..."
LangString THRM_STR_PAWNIO_FALLBACK ${LANG_ENGLISH} "PawnIO silent install/update failed. Switching to interactive installation..."
LangString THRM_STR_PAWNIO_FALLBACK ${LANG_JAPANESE} "PawnIO のサイレントインストール/更新に失敗したため、対話式インストールに切り替えます..."

LangString THRM_STR_UNINSTALL_STOP ${LANG_SIMPCHINESE} "正在停止运行中的进程..."
LangString THRM_STR_UNINSTALL_STOP ${LANG_ENGLISH} "Stopping running processes..."
LangString THRM_STR_UNINSTALL_STOP ${LANG_JAPANESE} "実行中のプロセスを停止しています..."

LangString THRM_STR_STOP_CORE ${LANG_SIMPCHINESE} "正在停止 FanControlPortable Core.exe..."
LangString THRM_STR_STOP_CORE ${LANG_ENGLISH} "Stopping FanControlPortable Core.exe..."
LangString THRM_STR_STOP_CORE ${LANG_JAPANESE} "FanControlPortable Core.exe を停止しています..."

LangString THRM_STR_STOP_LEGACY_CORE ${LANG_SIMPCHINESE} "正在停止历史核心服务..."
LangString THRM_STR_STOP_LEGACY_CORE ${LANG_ENGLISH} "Stopping previous core service..."
LangString THRM_STR_STOP_LEGACY_CORE ${LANG_JAPANESE} "以前のコアサービスを停止しています..."

LangString THRM_STR_STOP_APP ${LANG_SIMPCHINESE} "正在停止 ${PRODUCT_EXECUTABLE}..."
LangString THRM_STR_STOP_APP ${LANG_ENGLISH} "Stopping ${PRODUCT_EXECUTABLE}..."
LangString THRM_STR_STOP_APP ${LANG_JAPANESE} "${PRODUCT_EXECUTABLE} を停止しています..."

LangString THRM_STR_STOP_BRIDGE ${LANG_SIMPCHINESE} "正在停止 FanControlPortable TempBridge.exe..."
LangString THRM_STR_STOP_BRIDGE ${LANG_ENGLISH} "Stopping FanControlPortable TempBridge.exe..."
LangString THRM_STR_STOP_BRIDGE ${LANG_JAPANESE} "FanControlPortable TempBridge.exe を停止しています..."

LangString THRM_STR_STOP_LEGACY_BRIDGE ${LANG_SIMPCHINESE} "正在停止 TempBridge.exe..."
LangString THRM_STR_STOP_LEGACY_BRIDGE ${LANG_ENGLISH} "Stopping TempBridge.exe..."
LangString THRM_STR_STOP_LEGACY_BRIDGE ${LANG_JAPANESE} "TempBridge.exe を停止しています..."

LangString THRM_STR_REMOVE_AUTOSTART ${LANG_SIMPCHINESE} "正在移除自启动项..."
LangString THRM_STR_REMOVE_AUTOSTART ${LANG_ENGLISH} "Removing auto-start entries..."
LangString THRM_STR_REMOVE_AUTOSTART ${LANG_JAPANESE} "自動起動設定を削除しています..."

LangString THRM_STR_REMOVE_APPDATA ${LANG_SIMPCHINESE} "正在移除应用数据..."
LangString THRM_STR_REMOVE_APPDATA ${LANG_ENGLISH} "Removing application data..."
LangString THRM_STR_REMOVE_APPDATA ${LANG_JAPANESE} "アプリケーションデータを削除しています..."

LangString THRM_STR_REMOVE_INSTALL_FILES ${LANG_SIMPCHINESE} "正在移除安装文件..."
LangString THRM_STR_REMOVE_INSTALL_FILES ${LANG_ENGLISH} "Removing installed files..."
LangString THRM_STR_REMOVE_INSTALL_FILES ${LANG_JAPANESE} "インストール済みファイルを削除しています..."

LangString THRM_STR_REMOVE_BRIDGE ${LANG_SIMPCHINESE} "正在删除桥接组件..."
LangString THRM_STR_REMOVE_BRIDGE ${LANG_ENGLISH} "Removing bridge components..."
LangString THRM_STR_REMOVE_BRIDGE ${LANG_JAPANESE} "ブリッジコンポーネントを削除しています..."

LangString THRM_STR_REMOVE_LOGS ${LANG_SIMPCHINESE} "正在删除日志文件..."
LangString THRM_STR_REMOVE_LOGS ${LANG_ENGLISH} "Removing log files..."
LangString THRM_STR_REMOVE_LOGS ${LANG_JAPANESE} "ログファイルを削除しています..."

LangString THRM_STR_REMOVE_DIR ${LANG_SIMPCHINESE} "正在删除安装目录..."
LangString THRM_STR_REMOVE_DIR ${LANG_ENGLISH} "Removing installation directory..."
LangString THRM_STR_REMOVE_DIR ${LANG_JAPANESE} "インストールディレクトリを削除しています..."

LangString THRM_STR_REMOVE_SHORTCUTS ${LANG_SIMPCHINESE} "正在移除快捷方式..."
LangString THRM_STR_REMOVE_SHORTCUTS ${LANG_ENGLISH} "Removing shortcuts..."
LangString THRM_STR_REMOVE_SHORTCUTS ${LANG_JAPANESE} "ショートカットを削除しています..."

LangString THRM_STR_UNINSTALL_COMPLETE ${LANG_SIMPCHINESE} "卸载完成"
LangString THRM_STR_UNINSTALL_COMPLETE ${LANG_ENGLISH} "Uninstallation complete"
LangString THRM_STR_UNINSTALL_COMPLETE ${LANG_JAPANESE} "アンインストールが完了しました"

!endif
