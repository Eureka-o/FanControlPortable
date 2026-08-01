# FanControl 2.7.2

## 历史曲线显示
### History Chart Display

- 移除曲线上的突变点标记，减少不必要的视觉干扰。 Removed abrupt-change markers to keep history charts easier to read.
- 最大值和最小值仅在单数据系列显示时呈现，多数据对比时保持清爽的曲线视图。 Maximum and minimum markers now appear only for a single visible series.

## 稳定性与资源占用
### Stability and Resource Usage

- 配置保存、设备连接和挂起恢复后的状态同步更加稳定。 More reliable configuration saves and device state synchronization after reconnect and resume.
- 减少后台重复读取和重复处理，降低不必要的资源占用。 Reduced redundant background reads and processing for lower overhead.

## 主题与字体兼容性
### Theme and Font Compatibility

- 修复蜡笔小新主题与小八 Plus 主题中“总”字缺失的问题，补齐对应的圆润中文字体显示。 Restored the missing “总” glyph in the Crayon Shin-chan and Xiaoba Plus themes.
- 统一自定义主题缓存和 FanControl 主题标识，同时保留旧主题缓存的兼容读取。 Custom theme caching now uses FanControl identifiers while retaining legacy cache compatibility.
- 两个主题的字体资源与主题版本已同步更新，安装升级后会自动刷新内置主题文件。 Updated theme font assets and versions are refreshed automatically during installation upgrades.

## 兼容性说明
### Compatibility Notes

- 可直接覆盖安装旧版本；设备配置、风扇曲线、学习数据、历史记录、主题、托盘设置和 IP 设置会继续保留。 You can install over previous versions; device settings, fan curves, learning data, history, themes, tray settings, and IP settings are preserved.
