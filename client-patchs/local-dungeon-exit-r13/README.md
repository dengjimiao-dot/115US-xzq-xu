# 本地副本退出候选 r13

业主已明确授权本地版允许中途回城和结束游戏。最终调律者 PVF 的 `[disable exit] 2` 和原生副本结束游戏谓词会禁用菜单，服务端补包无法替代该本地策略调整。

插件只调整菜单动作 22、23、30 的权限，保留原生确认和执行流程。组队时的“退出队伍”使用动作 30，原生执行器与动作 22 都进入回城确认流程；上一版遗漏了此入口。仅原生普通副本状态 3、淡入结束后的 idle 阶段 0、当前对象一致且玩家/副本存在时放行。加载过渡、城镇和其他模式保持原生规则。不同客户端或函数字节不符时不写入。

通过 schema 2 mod 安装到 `.115us-mods/LocalDungeonExit.dll`，依赖既有 `qol.client-host`。只有候选启动入口设置 `DFO_LOCAL_DUNGEON_EXIT=r13-local-exit-v1` 时生效；其他入口不生效。日志位于 DLL 同目录 `local-dungeon-exit.log`。

不覆盖 DFO.exe 或 PVF。退出游戏后使用 modkit uninstall --id qol.local-dungeon-exit 卸载；内存改动随进程退出消失。自动验证不等于实机通过，需业主测试最终调律者和普通副本的回城、结束游戏及再次组队。
