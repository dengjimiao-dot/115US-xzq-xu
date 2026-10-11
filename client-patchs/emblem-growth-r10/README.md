# 徽章与星徽养成 r10：源码、资源与配套 DLL 候选

本候选接续 R7 非团本分支，并合入最新上游。分解、突破、共鸣、兑换和变更由 Go 读取 PVF 内容定义，经过物品资格、材料、概率、事务、幂等回执和存档校验后执行。客户端 DLL 负责原生窗口、拖放、动画、中文结果与浮窗；这些进程内绘制行为不能由服务端替代。

这是源码和离线机制检查通过的候选，尚未取得本组合的实机验收。不要将此前 r9 的部分验收或本地原 r10 安装记录视为这个 Git 组合已经通过游戏验证。

## 配套目录

- `../local-features/src/`：原生 NPC 入口、窗口生命周期、徽章拖放与资格、结果布局、动画、浮窗和属性行源码及测试。
- `../../server/work/dfo-lan/`：徽章养成 reader、领域规则、执行、协议桥接与 SQLite 公共材料增量迁移。
- `resources/records/`、`resources/stages/`：四次资源变更的逻辑记录及本地化文本。原生道具索引只保存追加的 544 对，保留已有 200379 对原始字节。
- `resources/images/`、`resources/image-packs.json`：71 个原始 IMG 载荷、三个 NPK 的布局和索引变更；包含动画真实引用的图层，未用占位图片替代。
- `../r7-nonraid-update/payload/client/`：同库提交的九个 DLL 和插件加载清单。`paired-manifest.json` 校验载荷、源文件和必需资源。
- `../../scripts/emblem-growth-r10/`：资源重建、校验、绑定、配对清单刷新和独立启动 profile 准备工具。

完整的 PVF 和 NPK 是重建输出，留在忽略的临时目录。库内单个资源源文件均小于 5 MiB。重建需要已有 R7 累计资源，基线指纹由 stages/r2.json 和 image-packs.json 强制校验；不是从任意原版 PVF 套用。

## 重建与配对

需要 Python 3.10+、PowerShell 7、MSVC x64（静态 CRT）、Windows SDK 和支持本方案接口的 `USLocalServer.Pvf.dll` 编辑工具。PVF 编辑工具和原始 DFO.exe 为外部构建依赖，不随源码重新分发。全部命令在仓库根执行，尖括号替换为本机实际路径。

```powershell
python scripts/emblem-growth-r10/build-paired-resources.py --base-client '<R7累计资源客户端>' --exe '<DFO.exe>' --editor-dll '<USLocalServer.Pvf.dll>' --pwsh '<pwsh.exe>' --out '<新的空资源输出目录>'
python scripts/emblem-growth-r10/refresh-pair.py --resources '<资源输出目录>'
```

第二条只检查现有源码绑定是否匹配。资源指纹不同但语义验证通过时，先显式重绑，再编译，再刷新配对清单；不能仅修改清单冒充重新编译：

```powershell
python scripts/emblem-growth-r10/refresh-pair.py --resources '<资源输出目录>' --bind
cmd /c client-patchs\local-features\build.cmd
python scripts/emblem-growth-r10/refresh-pair.py --resources '<资源输出目录>' --refresh-payload
```

本提交也重新编译了 R7 代理、退出插件及两组旧测试指纹过期的 DLL。需要刷新这些 DLL 时，先运行各自的 `build-marker.cmd` / `test-regression.cmd`、`build-local-exit.cmd`、`build-candidate.cmd`、`build-native-ai.cmd`，再在刷新命令中按需追加 `--additional-target dinput8.dll`、`--additional-target .115us-mods/LocalDungeonExit.dll` 等。其余插件沿用源码一致的 R7 产物。

构建脚本支持 `VCVARS64`、`US115_SDK_ROOT` 和 `US115_SDK_VERSION`。资源编辑器验证四阶段逻辑记录、字符串池前缀、原有压缩数据和分组元数据、每个声明修改与原生索引追加；图片重建验证整包 SHA256 和索引，保留其它 14492 个索引条目。

## 服务端与安装回退

先从本分支源码编译网关，再生成独立 profile。该工具复制重建的服务端归档并只改 binary、DFO_PVF_ARCHIVE、DFO_PVF_SHA256；保留既有团本、策略、catalogs 和其它环境设置，不读取玩家存档，不修改原启动 profile。

```powershell
# 在 server/work/dfo-lan 下构建，产物不要提交。
go build -o bin/wireprobe-emblem-r10.exe ./cmd/wireprobe
# 回到仓库根。
python scripts/emblem-growth-r10/prepare-profile.py --resources '<资源输出目录>' --base-profile '<现有profile.json>' --binary '<新编译的网关exe>' --out '<新的空候选服务目录>'
python client-patchs/r7-nonraid-update/build-mod.py --resources '<资源输出目录>' --client '<客户端目录>' --out '<临时目录>/emblem-r10.mod.zip'
modkit verify --mod '<临时目录>/emblem-r10.mod.zip'
modkit plan --root '<运行包目录>' --client '<客户端目录>' --mod '<临时目录>/emblem-r10.mod.zip'
```

包内有 16 个 client 层目标（九个 DLL、插件清单、PVF、密钥、图片索引和三个 NPK），以及 `paired-server/Script.inner.pvf`。服务端归档不是 server hook，不会被 modkit 自动安装；必须使用上一条生成的同指纹 profile 并在启动器中选择它。仅安装 client 包而使用旧网关或旧服务端归档是不完整的更新。

正常退出目标游戏和服务端，审阅计划。已有 mod 占用相同文件时先通过原 modkit 流程卸载或按原所有权升级；不改注册表，不覆盖原所有者的备份。有自定义 plugins.json 时先合并保留声明、更新指纹，再打包。

```powershell
modkit install --root '<运行包目录>' --client '<客户端目录>' --mod '<临时目录>/emblem-r10.mod.zip'
# 选择生成的 profile 后由玩家进行游戏验收。
# 回退：退出后卸载 client 包，并在启动器选择此前 profile。
modkit uninstall --root '<运行包目录>' --client '<客户端目录>' --id qol.r7-nonraid-client
```

公共材料新增独立 SQLite 表；旧材料文档和既有迁移分节没有改写。升级校验原迁移账本，旧版服务端写回旧表不清除新增公共材料。原有事务回执、未知存档字段、账号共享与正常突破进度保留由回归测试覆盖。不要随包提交 SQLite、日志、缓存、账号或测试发放物品记录。

## 待玩家验证

1. 达芙妮三个入口重复打开、关闭和切换，图标与状态正常。
2. 分解和突破同时打开背包徽章页，拖放有图标，不合资格徽章变暗；材料不足有提示。
3. 分解动画完整播放后出现结果；徽章全名、各项数量和中文完整显示，批量分解不重放过期结果。
4. 普通突破取下、重放和重开窗口后保留进度。
5. 共鸣浮窗跟随鼠标，边界不越屏，动画和中文结果正常；材料不足不伪装为成功。
6. 综合属性按品质着色，共鸣效果完整换行；移除、失效和重复请求不造成属性叠加或二次扣料。

资源索引的原生前缀保持策略使此组合 PVF 的字节指纹不同于本地旧 r10。两处 DLL 绑定已按重建结果更新，实际客户端消费仍需以上验收。
