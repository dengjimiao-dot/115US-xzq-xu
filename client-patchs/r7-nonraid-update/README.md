# R7 非团本功能配套更新

本目录把 9 个 DLL、插件加载清单、假人启动配置示例与对应源码放在同一 Git 提交中。DLL 已由本次分支源码编译，源文件和载荷 SHA256 见 `paired-manifest.json`。这是基于最新上游的候选组合，尚未进行合并后实机验收。

## 配套文件

`payload/client/` 对应游戏客户端目录，包含 `dinput8.dll`、上游 1.1.0 宿主 `ChineseLocalization.dll`，以及 `.115us-mods/` 下的 LocalFeatures、EliteCompanionPilot、WeaponSkinAnimation、HeadTextUpscale、LocalDungeonExit、RobotObjectAuthority、RobotNativeAI 和 `plugins.json`。

服务端不能替代这些已有进程内 UI、绘图、动画和假人控制钩子。宿主仍占用既有插件通道；插件路径相对 `.115us-mods/`，RobotNativeAI 声明依赖 RobotObjectAuthority。AI 在清单中先被访问，以便宿主同步初始化它的依赖。

适用客户端指纹为 `515fefa1c71577a15f90513f860270320f3502bc492ad596f10cf176b91c21eb`。源码保留现场字节检查；不同客户端应先核对源绑定与钩子，不能将候选的机制测试解释为跨版本兼容证明。所有 DLL 为 x64，静态 CRT。

## 安装与回退

先正常退出游戏。使用 Python 生成 schema 2 包，`--client` 只读取现有目标文件以选择 add/replace：

```powershell
python client-patchs/r7-nonraid-update/build-mod.py --client '<客户端目录>' --out '<临时目录>/r7-nonraid-client.mod.zip'
modkit verify --mod '<临时目录>/r7-nonraid-client.mod.zip'
modkit plan --root '<运行包目录>' --client '<客户端目录>' --mod '<临时目录>/r7-nonraid-client.mod.zip'
modkit install --root '<运行包目录>' --client '<客户端目录>' --mod '<临时目录>/r7-nonraid-client.mod.zip'
modkit uninstall --root '<运行包目录>' --client '<客户端目录>' --id qol.r7-nonraid-client
```

已有单独 mod 注册了相同目标时，先通过各自的 modkit 卸载流程处理文件所有权；不要直接覆盖注册表。组合包 ID 为 `qol.r7-nonraid-client`，包含宿主自身，不再单独依赖另一个宿主安装包。`plugins.json` 的替换同样会备份并可恢复；有自定义插件声明时先合并保留它们，再更新载荷指纹和打包。

假人开关还依赖运行包的 `release-manifest.json.local_features.robots` 及模板。先审阅计划，再补缺失配置：

```powershell
python client-patchs/r7-nonraid-update/prepare-launch-config.py --root '<运行包目录>'
python client-patchs/r7-nonraid-update/prepare-launch-config.py --root '<运行包目录>' --apply
```

此脚本保留已有设置、团本配置、模板及存档，写入前备份已有 release manifest。空模板不含角色数据；服务端按当前 PVF 初始化假人。启动器首次启动只在目标假人存档不存在时复制模板。服务端 profile 的非团本开关已在 `configs/pvf-default.json` 补齐。

## 构建与验证

各 DLL 来源目录见配对清单，运行其中的 `build-*.cmd`；本次新增脚本及宿主脚本支持 `VCVARS64` 指定 MSVC，`US115_SDK_ROOT` / `US115_SDK_VERSION` 指定可选 SDK。产物仍在忽略的 `dist/`。源绑定头已经提供，重新取证脚本要求显式传入客户端路径。

已在合成目录验证全新安装和替换 10 个已有目标，两条路径均通过 verify、plan、install、uninstall，卸载逐字节恢复。各插件的机制测试通过。这些检查没有启动 DFO，也没有读取玩家数据库。运行仍需要现有游戏资源、PVF、Go 服务端和配套 GUI；完整资源包不包含在本目录中。
