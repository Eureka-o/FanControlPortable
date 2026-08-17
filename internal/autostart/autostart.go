//go:build windows

// Package autostart 提供 Windows 自启动管理功能
package autostart

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"

	"github.com/Eureka-o/FanControlPortable/internal/appmeta"
	"github.com/Eureka-o/FanControlPortable/internal/types"
	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

// Manager 自启动管理器
type Manager struct {
	logger types.Logger
}

// NewManager 创建新的自启动管理器
func NewManager(logger types.Logger) *Manager {
	return &Manager{
		logger: logger,
	}
}

// IsRunningAsAdmin 检查是否以管理员权限运行
func (m *Manager) IsRunningAsAdmin() bool {
	var sid *windows.SID

	// 创建管理员组的SID
	err := windows.AllocateAndInitializeSid(
		&windows.SECURITY_NT_AUTHORITY,
		2,
		windows.SECURITY_BUILTIN_DOMAIN_RID,
		windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0,
		&sid)
	if err != nil {
		m.logger.Error("创建管理员SID失败: %v", err)
		return false
	}
	defer windows.FreeSid(sid)

	// 检查当前进程令牌
	token := windows.Token(0)
	member, err := token.IsMember(sid)
	if err != nil {
		m.logger.Error("检查管理员权限失败: %v", err)
		return false
	}

	return member
}

// SetWindowsAutoStart 设置Windows开机自启动
func (m *Manager) SetWindowsAutoStart(enable bool) error {
	// 检查是否以管理员权限运行
	if !m.IsRunningAsAdmin() {
		return fmt.Errorf("设置自启动需要管理员权限")
	}

	if enable {
		// 使用任务计划程序设置自启动
		return m.createScheduledTask()
	} else {
		// 删除任务计划程序和注册表项
		m.deleteScheduledTask()
		return m.removeRegistryAutoStart()
	}
}

// autoStartTargetPath 返回自启动应当拉起的可执行文件：优先核心服务，找不到时退回当前程序。
func autoStartTargetPath() (string, error) {
	exePath, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("获取程序路径失败: %v", err)
	}
	if corePath := appmeta.FirstExistingPath(appmeta.CoreExecutableCandidates(filepath.Dir(exePath))); corePath != "" {
		return corePath, nil
	}
	return exePath, nil
}

// currentUserAccount 返回用于任务主体的账户标识，优先当前进程令牌的 SID。
// SID 不依赖域控解析，离线域账号机器也能注册任务。
func currentUserAccount() string {
	if user, err := windows.GetCurrentProcessToken().GetTokenUser(); err == nil && user != nil {
		if sid := user.User.Sid; sid != nil {
			return sid.String()
		}
	}
	if domain, name := os.Getenv("USERDOMAIN"), os.Getenv("USERNAME"); name != "" {
		if domain != "" {
			return domain + `\` + name
		}
		return name
	}
	return ""
}

// buildScheduledTaskXML 是自启动任务的唯一 XML 定义来源。
func buildScheduledTaskXML(command, account string) string {
	principals := `
  <Principals>
    <Principal id="Author">` + taskUserIDElement(account, 6) + `
      <LogonType>InteractiveToken</LogonType>
      <RunLevel>HighestAvailable</RunLevel>
    </Principal>
  </Principals>`

	return `<?xml version="1.0" encoding="UTF-16"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
  <RegistrationInfo>
    <Author>` + escapeXML(appmeta.AppName) + `</Author>
    <Description>` + escapeXML(appmeta.AppName+" 开机自启动（登录后启动核心服务）") + `</Description>
  </RegistrationInfo>
  <Triggers>
    <LogonTrigger>
      <Enabled>true</Enabled>` + taskUserIDElement(account, 6) + `
    </LogonTrigger>
  </Triggers>` + principals + `
  <Settings>
    <MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy>
    <DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries>
    <StopIfGoingOnBatteries>false</StopIfGoingOnBatteries>
    <AllowHardTerminate>true</AllowHardTerminate>
    <StartWhenAvailable>true</StartWhenAvailable>
    <RunOnlyIfNetworkAvailable>false</RunOnlyIfNetworkAvailable>
    <IdleSettings>
      <StopOnIdleEnd>false</StopOnIdleEnd>
      <RestartOnIdle>false</RestartOnIdle>
    </IdleSettings>
    <AllowStartOnDemand>true</AllowStartOnDemand>
    <Enabled>true</Enabled>
    <Hidden>false</Hidden>
    <RunOnlyIfIdle>false</RunOnlyIfIdle>
    <WakeToRun>false</WakeToRun>
    <ExecutionTimeLimit>PT0S</ExecutionTimeLimit>
    <Priority>6</Priority>
    <RestartOnFailure>
      <Interval>PT1M</Interval>
      <Count>3</Count>
    </RestartOnFailure>
  </Settings>
  <Actions Context="Author">
    <Exec>
      <Command>` + escapeXML(command) + `</Command>
      <Arguments>--autostart</Arguments>
    </Exec>
  </Actions>
</Task>
`
}

func taskUserIDElement(account string, indent int) string {
	if account == "" {
		return ""
	}
	return "\n" + strings.Repeat(" ", indent) + "<UserId>" + escapeXML(account) + "</UserId>"
}

func escapeXML(value string) string {
	var buf strings.Builder
	if err := xml.EscapeText(&buf, []byte(value)); err != nil {
		return value
	}
	return buf.String()
}

// createScheduledTask 创建任务计划程序
func (m *Manager) createScheduledTask() error {
	command, err := autoStartTargetPath()
	if err != nil {
		return err
	}

	xmlPath, cleanup, err := writeTaskXMLFile(buildScheduledTaskXML(command, currentUserAccount()))
	if err != nil {
		return err
	}
	defer cleanup()

	cmd := exec.Command("schtasks", "/create", "/tn", appmeta.AppName, "/xml", xmlPath, "/f")

	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("创建任务计划失败: %v, 输出: %s", err, decodeSchtasksOutput(output))
	}

	m.logger.Info("已通过任务计划程序设置开机自启动（电池供电下同样启动，且不限运行时长）")
	return nil
}

func writeTaskXMLFile(content string) (string, func(), error) {
	file, err := os.CreateTemp("", "fancontrol-autostart-*.xml")
	if err != nil {
		return "", func() {}, fmt.Errorf("创建任务定义临时文件失败: %v", err)
	}
	path := file.Name()
	cleanup := func() { _ = os.Remove(path) }

	encoded := utf16.Encode([]rune(content))
	buf := make([]byte, 0, len(encoded)*2+2)
	buf = append(buf, 0xFF, 0xFE)
	for _, unit := range encoded {
		buf = append(buf, byte(unit), byte(unit>>8))
	}
	if _, err := file.Write(buf); err != nil {
		_ = file.Close()
		cleanup()
		return "", func() {}, fmt.Errorf("写入任务定义失败: %v", err)
	}
	if err := file.Close(); err != nil {
		cleanup()
		return "", func() {}, fmt.Errorf("关闭任务定义文件失败: %v", err)
	}
	return path, cleanup, nil
}

func decodeSchtasksOutput(output []byte) string {
	if len(output) >= 2 && output[0] == 0xFF && output[1] == 0xFE {
		units := make([]uint16, 0, (len(output)-2)/2)
		for i := 2; i+1 < len(output); i += 2 {
			units = append(units, uint16(output[i])|uint16(output[i+1])<<8)
		}
		return string(utf16.Decode(units))
	}
	return string(output)
}

type scheduledTaskSettings struct {
	Settings struct {
		DisallowStartIfOnBatteries *bool  `xml:"DisallowStartIfOnBatteries"`
		StopIfGoingOnBatteries     *bool  `xml:"StopIfGoingOnBatteries"`
		ExecutionTimeLimit         string `xml:"ExecutionTimeLimit"`
	} `xml:"Settings"`
}

func trimUnicodeBOM(data []byte) []byte {
	return bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
}

func parseTaskDefinition(definition string, out any) error {
	decoder := xml.NewDecoder(bytes.NewReader(trimUnicodeBOM([]byte(definition))))
	decoder.CharsetReader = func(_ string, input io.Reader) (io.Reader, error) {
		return input, nil
	}
	return decoder.Decode(out)
}

func scheduledTaskNeedsUpgrade(definition string) bool {
	var parsed scheduledTaskSettings
	if err := parseTaskDefinition(definition, &parsed); err != nil {
		return false
	}
	settings := parsed.Settings
	if settings.DisallowStartIfOnBatteries == nil || *settings.DisallowStartIfOnBatteries {
		return true
	}
	if settings.StopIfGoingOnBatteries == nil || *settings.StopIfGoingOnBatteries {
		return true
	}
	return strings.TrimSpace(settings.ExecutionTimeLimit) != "PT0S"
}

// EnsureAutoStartTaskHealthy 将旧版命令行任务升级到当前 XML 定义。
func (m *Manager) EnsureAutoStartTaskHealthy() (bool, error) {
	taskName := m.scheduledTaskName()
	if taskName == "" {
		return false, nil
	}

	cmd := exec.Command("schtasks", "/query", "/tn", taskName, "/xml")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	output, err := cmd.Output()
	if err != nil {
		return false, fmt.Errorf("查询自启动任务定义失败: %v", err)
	}
	if !scheduledTaskNeedsUpgrade(decodeSchtasksOutput(output)) {
		return false, nil
	}
	if !m.IsRunningAsAdmin() {
		m.logger.Info("自启动任务仍是旧定义，需以管理员身份运行一次以自动修正")
		return false, nil
	}

	m.logger.Info("检测到旧版自启动任务定义，正在重建")
	if err := m.createScheduledTask(); err != nil {
		return false, err
	}
	if !strings.EqualFold(taskName, appmeta.AppName) {
		if err := m.deleteScheduledTaskByName(taskName); err != nil {
			return true, fmt.Errorf("删除旧版自启动任务失败: %v", err)
		}
	}
	return true, nil
}

// deleteScheduledTask 删除任务计划程序
func (m *Manager) deleteScheduledTask() error {
	for _, taskName := range []string{appmeta.AppName, appmeta.LegacyAppName} {
		if err := m.deleteScheduledTaskByName(taskName); err != nil {
			return err
		}
	}

	m.logger.Info("Deleted auto-start scheduled tasks")
	return nil
}

func (m *Manager) deleteScheduledTaskByName(taskName string) error {
	cmd := exec.Command("schtasks", "/delete", "/tn", taskName, "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.CombinedOutput()
	if err != nil {
		decoded := decodeSchtasksOutput(output)
		if strings.Contains(decoded, "cannot be found") || strings.Contains(decoded, "不存在") {
			return nil
		}
		return fmt.Errorf("delete scheduled task failed: %v, output: %s", err, decoded)
	}
	return nil
}

// removeRegistryAutoStart 删除注册表自启动项
func (m *Manager) removeRegistryAutoStart() error {
	var firstErr error
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
		if err != nil {
			if root == registry.LOCAL_MACHINE {
				continue
			}
			return fmt.Errorf("open registry failed: %v", err)
		}

		for _, valueName := range []string{appmeta.AppName, appmeta.LegacyAppName} {
			err = key.DeleteValue(valueName)
			if err != nil && err != registry.ErrNotExist && firstErr == nil {
				firstErr = err
			}
		}
		key.Close()
	}

	if firstErr != nil {
		return fmt.Errorf("delete registry value failed: %v", firstErr)
	}

	m.logger.Info("Deleted registry auto-start values")
	return nil
}

// GetAutoStartMethod 获取当前的自启动方式
func (m *Manager) GetAutoStartMethod() string {
	if m.checkScheduledTask() {
		return "task_scheduler"
	}
	if m.checkRegistryAutoStart() {
		return "registry"
	}
	return "none"
}

// SetAutoStartWithMethod 使用指定方式设置自启动
func (m *Manager) SetAutoStartWithMethod(enable bool, method string) error {
	if !enable {
		m.deleteScheduledTask()
		m.removeRegistryAutoStart()
		return nil
	}

	switch method {
	case "task_scheduler":
		if !m.IsRunningAsAdmin() {
			return fmt.Errorf("使用任务计划程序需要管理员权限，请以管理员身份运行程序进行设置")
		}
		return m.createScheduledTask()

	case "registry":
		return m.setRegistryAutoStart()

	default:
		return fmt.Errorf("不支持的自启动方式: %s", method)
	}
}

// setRegistryAutoStart 设置注册表自启动
func (m *Manager) setRegistryAutoStart() error {
	key, err := registry.OpenKey(registry.CURRENT_USER, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("打开注册表失败: %v", err)
	}
	defer key.Close()

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %v", err)
	}
	exeDir := filepath.Dir(exePath)
	corePath := appmeta.FirstExistingPath(appmeta.CoreExecutableCandidates(exeDir))

	// 如果核心服务不存在，使用当前程序路径
	if corePath == "" {
		corePath = exePath
	}
	exePathWithArgs := fmt.Sprintf("\"%s\" --autostart", corePath)

	err = key.SetStringValue(appmeta.AppName, exePathWithArgs)
	if err != nil {
		return fmt.Errorf("设置注册表失败: %v", err)
	}

	m.logger.Info("已通过注册表设置开机自启动")
	return nil
}

// CheckWindowsAutoStart 检查Windows开机自启动状态
func (m *Manager) CheckWindowsAutoStart() bool {
	if m.checkScheduledTask() {
		return true
	}

	return m.checkRegistryAutoStart()
}

// checkScheduledTask 检查任务计划程序中的自启动任务
func (m *Manager) checkScheduledTask() bool {
	return m.scheduledTaskName() != ""
}

func (m *Manager) scheduledTaskName() string {
	for _, taskName := range []string{appmeta.AppName, appmeta.LegacyAppName} {
		cmd := exec.Command("schtasks", "/query", "/tn", taskName, "/fo", "list", "/v")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

		output, err := cmd.Output()
		if err != nil {
			continue
		}
		command := scheduledTaskCommandLine(decodeSchtasksOutput(output))
		if command == "" || commandTargetsFanControl(command) {
			return taskName
		}
	}
	return ""
}

// checkRegistryAutoStart 检查注册表中的自启动项
func (m *Manager) checkRegistryAutoStart() bool {
	for _, root := range []registry.Key{registry.CURRENT_USER, registry.LOCAL_MACHINE} {
		key, err := registry.OpenKey(root, `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`, registry.QUERY_VALUE)
		if err != nil {
			m.logger.Debug("open registry failed: %v", err)
			continue
		}
		for _, valueName := range []string{appmeta.AppName, appmeta.LegacyAppName} {
			value, _, err := key.GetStringValue(valueName)
			if err != nil {
				continue
			}
			if commandTargetsFanControl(value) {
				key.Close()
				return true
			}
		}
		key.Close()
	}
	return false
}
func scheduledTaskCommandLine(output string) string {
	for line := range strings.SplitSeq(output, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		for _, prefix := range []string{"Task To Run:", "要运行的任务:", "実行するタスク:"} {
			if after, ok := strings.CutPrefix(line, prefix); ok {
				return strings.TrimSpace(after)
			}
		}
	}
	return ""
}

func commandTargetsCurrentCore(command string) bool {
	target := firstCommandPath(command)
	if target == "" {
		return false
	}
	expected := currentCorePath()
	if expected == "" {
		return false
	}
	targetAbs, err := filepath.Abs(target)
	if err == nil {
		target = targetAbs
	}
	expectedAbs, err := filepath.Abs(expected)
	if err == nil {
		expected = expectedAbs
	}
	return strings.EqualFold(filepath.Clean(target), filepath.Clean(expected))
}

func commandTargetsFanControl(command string) bool {
	target := firstCommandPath(command)
	if target == "" {
		return false
	}
	if commandTargetsCurrentCore(command) {
		return true
	}
	base := filepath.Base(target)
	if !isFanControlExecutableName(base) {
		return false
	}
	cleanTarget := strings.ToLower(filepath.ToSlash(filepath.Clean(target)))
	for _, marker := range []string{
		"/fancontrol/",
		"/fancontrolportable/",
		"/eureka-o fancontrol/",
	} {
		if strings.Contains(cleanTarget, marker) {
			return true
		}
	}
	return false
}

func isFanControlExecutableName(name string) bool {
	for _, expected := range []string{
		appmeta.CoreExecutableName,
		appmeta.LegacyCoreExecutable,
		appmeta.ExecutableName,
		appmeta.LegacyExecutableName,
	} {
		if strings.EqualFold(name, expected) {
			return true
		}
	}
	return false
}

func firstCommandPath(command string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		return ""
	}
	if strings.HasPrefix(command, `"`) {
		rest := strings.TrimPrefix(command, `"`)
		if idx := strings.Index(rest, `"`); idx >= 0 {
			return rest[:idx]
		}
		return rest
	}
	fields := strings.Fields(command)
	if len(fields) == 0 {
		return ""
	}
	return fields[0]
}

func currentCorePath() string {
	exePath, err := os.Executable()
	if err != nil {
		return ""
	}
	exeDir := filepath.Dir(exePath)
	corePath := appmeta.FirstExistingPath(appmeta.CoreExecutableCandidates(exeDir))
	if corePath == "" {
		return exePath
	}
	return corePath
}

// DetectAutoStartLaunch 检测是否为自启动启动
func DetectAutoStartLaunch(args []string) bool {
	for _, arg := range args {
		if arg == "--autostart" || arg == "/autostart" || arg == "-autostart" {
			return true
		}
	}

	if isLaunchedByTaskScheduler() {
		return true
	}

	// 检查当前工作目录是否为系统目录
	wd, err := os.Getwd()
	if err == nil {
		systemDirs := []string{
			"C:\\Windows\\System32",
			"C:\\Windows\\SysWOW64",
			"C:\\Windows",
		}

		for _, sysDir := range systemDirs {
			if strings.EqualFold(wd, sysDir) {
				return true
			}
		}
	}

	return false
}

// isLaunchedByTaskScheduler 检查是否由任务计划程序启动
func isLaunchedByTaskScheduler() bool {
	// 在Windows上检查父进程
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", os.Getpid()), "get", "ParentProcessId", "/value")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "ParentProcessId="); ok {
			ppidStr := strings.TrimSpace(after)
			if ppidStr != "" && ppidStr != "0" {
				ppid, err := parseIntSafe(ppidStr)
				if err == nil {
					return checkParentProcessName(ppid)
				}
			}
		}
	}

	return false
}

// checkParentProcessName 检查父进程名称
func checkParentProcessName(ppid int) bool {
	cmd := exec.Command("wmic", "process", "where", fmt.Sprintf("ProcessId=%d", ppid), "get", "Name", "/value")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	output, err := cmd.Output()
	if err != nil {
		return false
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		line = strings.TrimSpace(line)
		if after, ok := strings.CutPrefix(line, "Name="); ok {
			processName := strings.ToLower(strings.TrimSpace(after))
			// 检查是否为任务计划程序相关进程
			if processName == "taskeng.exe" || processName == "svchost.exe" || processName == "taskhostw.exe" {
				return true
			}
		}
	}

	return false
}

// parseIntSafe 安全解析整数
func parseIntSafe(s string) (int, error) {
	var result int
	_, err := fmt.Sscanf(s, "%d", &result)
	return result, err
}
