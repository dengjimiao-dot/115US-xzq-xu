# 日服语音补丁

此补丁替换客户端 SoundPacks 中的角色、NPC 和怪物语音。服务端不能改变客户端实际播放的音频，因此配套 NPK 与工具一同交付；无 DLL、EXE、PVF 或玩法修改。当前为待实机试听的候选，默认不开启。

主体为“日服语音补丁 - 狄瑞吉攻坚战版本.zip”；“DNF完美日服语音包.rar”的有效 Ogg 音频补齐主体缺项。重复名称优先使用主体，26 条非 Ogg 静音占位排除。韩服补充包未混入。源文件名、来源指纹、每个原资源及替换资源的 SHA256、替换数量见 `manifest.json`；原作者说明见 `source-notes.txt`。

共匹配 68,934 条已有音频，实际改变 31,242 条，输出 95 个原生 NPK，总大小 2,280,752,776 字节。保留原资源包条目名称、顺序及其余音频，不添加当前客户端不存在的资源键。95 个最终 NPK 通过本目录 `.gitattributes` 使用 Git LFS 保存：93 个直接保存，NPC 与 UI 两个超限文件无损拆分为各两段，共 97 个 LFS 对象。克隆后先运行 `git lfs pull`，再运行 `python client-patchs/japanese-voice/prepare-resources.py`，将两份大资源按清单逐段校验后还原；原始 NPK 的 SHA256 和安装/回退方式完全相同。否则得到的只是指针或未还原的资源。还原的两个 NPK 作为生成文件忽略，不重复入库。构建工具会检查完整资源指纹并拒绝指针、缺失或损坏的文件。

## 安装与恢复

仓库克隆后先取得并准备资源（Python 3.11+，只处理补丁目录）：

```powershell
git lfs pull
python client-patchs/japanese-voice/prepare-resources.py
```

已交付的独立补丁 ZIP 包含完整 NPK，可直接使用。

先正常退出全部 DFO/DNF 游戏。显式指定要安装的客户端：

```powershell
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/jp-voice.ps1 -Mode Check -ClientPath '<客户端目录>'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/jp-voice.ps1 -Mode Install -ClientPath '<客户端目录>'
powershell.exe -NoProfile -ExecutionPolicy Bypass -File scripts/jp-voice.ps1 -Mode Restore
```

也可将客户端文件夹拖到 `scripts/jp-voice-install.cmd` 上；检查及恢复入口分别为 `jp-voice-check.cmd` 和 `jp-voice-restore.cmd`。只读检查可在游戏运行时执行。仓库版不会自动选择机器上的客户端，主客户端和独立测试客户端需分别指定。

安装先核对全部目标与载荷、备份全部原文件，再逐文件原子替换，避免原地改变其它硬链接副本。备份位于本目录忽略的 `backups/`，最近安装指针在忽略的 `last-install.json`。指定某次备份恢复：`-Mode Restore -BackupPath '<备份目录>'`。损坏载荷、未知资源版本、额外语音覆盖包、部分安装状态、回退前第三方改动均停止覆盖；已开始写入的安装失败时自动恢复已写入文件。保留备份和补丁目录直到不再需要回退。

## mod 框架

以下打包只读检查客户端，不安装：

```powershell
python client-patchs/japanese-voice/build-mod.py --client '<原客户端目录>' --out '<临时目录>/japanese-voice.mod.zip'
modkit verify --mod '<临时目录>/japanese-voice.mod.zip'
modkit plan --root '<运行包目录>' --client '<客户端目录>' --mod '<临时目录>/japanese-voice.mod.zip'
modkit install --root '<运行包目录>' --client '<客户端目录>' --mod '<临时目录>/japanese-voice.mod.zip'
modkit uninstall --root '<运行包目录>' --client '<客户端目录>' --id qol.japanese-voice
```

安装方法二选一；不混用独立安装器与 modkit 的备份或文件所有权。构建为 schema 2、client 层 `file.replace`、权限 `client.file.write`。无需客户端 DLL 宿主。已有 mod 注册相同声音资源时，先按其正式卸载流程处理。

## 重建与验证

Python 3.11+ 和 7-Zip 可从已有源包重建资源；RAR 密码交互输入，也可通过仅在进程环境中设置的 `JPVOICE_RAR_PASSWORD` 提供。密码不写入源码或清单。

```powershell
python client-patchs/japanese-voice/build-resources.py --client '<原客户端目录>' --jp-zip '<日服ZIP>' --perfect-rar '<完美日服RAR>' --out '<新的构建目录>' --sevenzip '<7z.exe路径>'
python client-patchs/japanese-voice/verify-installer.py --client '<原客户端目录>' --out '<新的隔离验证目录>'
```

重建仅写指定输出目录，拒绝输出到客户端目录，保留工作文件于构建输出 `.build/`。全部替换音频对照来源逐字节检查，保留音频对照原包逐字节检查，索引签名重新计算并核对；源 ZIP/RAR 在解压读取时检查 CRC。安装器检查只在隔离夹具执行，原客户端仅用于读取基线。

`verification.json` 记录验证结果。音频实际播放效果仍需安装后由玩家手动试听，自动检查不作为实机验收。
