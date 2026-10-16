# 累计 R7 启动器与内嵌 GM 源码

GUI 2.0.7.0，当前为保留上游团本的 R7 非团本迁移候选，发布标识 `2026.10.11-r7-nonraid-upstream`。

在 Windows 上安装 .NET Framework 4.x 编译器和 WPF 组件，先在 `server/work/dfo-lan/` 编译：

```powershell
go build -o bin/gmtool-local.exe ./cmd/gmtool
go build -o bin/wireprobe-handoff-source.exe ./cmd/wireprobe
```

再从仓库根执行：

```powershell
pwsh -NoProfile -File launcher/Build.ps1 -Backend server/work/dfo-lan/bin/gmtool-local.exe -Server server/work/dfo-lan/bin/wireprobe-handoff-source.exe
& './launcher/dist/DFO-115US本地版登录器.exe' --self-test --evidence './launcher/self-test'
```

产物留在未跟踪的 `dist/`。源码检出不包含完整游戏运行包、资源、玩家数据库或默认服务端程序。正常启动需要与该冻结版本对应的运行包与 release manifest；不要把新编译程序直接覆盖到另一版本的运行包中。

R7 会实际计算发行文件哈希，记录不一致并继续启动；必须文件、资源解析、存档身份、安装载荷和回退校验仍独立执行。迁移范围见 [迁移说明](../server/work/dfo-lan/docs/r7-nonraid-upstream-migration.md)。配套 DLL、插件清单和假人配置准备脚本均包含在 [配套更新目录](../client-patchs/r7-nonraid-update/README.md)，必须与服务端一起更新。新上游组合仍按 Draft 评审，不以原累计包记录代替合并后实机验收。
