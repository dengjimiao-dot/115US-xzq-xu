# 巴卡尔基础战斗确认基线（2026-10-06）

用户确认：“现在小怪需要击杀才有效了……邪龙、狂龙及部分门将现在也可以打了”。

确认范围：小怪正常战斗/击杀、邪龙/狂龙及部分门将实际战斗对象生成，保留冰龙已可战斗的反馈。不将7组自动入口测试等同于7种首领全部实机验收，不将本轮确认扩大到免疫BUFF、随机觉醒、后续刷新、奖励或完整通关。

- 精确程序：`bin/wireprobe-bakal-live-arena-candidate.exe`。
- SHA256：`75427e459ed989330ea893143f29a06a2ecf1b4a9aa9700a74aa232526b079b2`。
- 实机记录：`runtime/roles_persist_select_actor_town_world_live_detail_dungeon_manual_20261006_162624_638704_next37`，用户最新反馈为验收依据。
- 客户端：`D:\115us\DFO`，启动路线PostgreSQL。
- 精确二进制与当时profile已保存到 `runtime/baselines/bakal-core-confirmed-20261006`，不进入Git。

确认后的机制审查发现波次怪物类型枚举有错误，新候选纠正斯万9/埃可莱尔11及其它原生类型；新构建必须与上述已确认程序区别记录，不能以源码再构建覆盖已确认指纹。BUFF与源计时事件缺口见 `bakal-mechanics-audit-20261006.md`。

收口提交限制：server/AGENTS.md要求提交前执行scripts/check-commit-hygiene.ps1；实际调用失败且全仓没有该脚本。本轮保留确认记录和变更，未跳过门禁提交。


## 2026-10-10：寻找攻坚队菜单门禁实机确认

业主“ok,完美了,现在”及截图3530e188确认：开战后可打开寻找攻坚队列表及当前团成员窗口。确认候选wireprobe-handoff-source.exe，SHA256 c452ee185636827151f222c414c18359ecdbe8dce7b32163ac9b4f353a252051。当前2.38.2.34原生NOTI1220/kind0启动controller+0x200的三秒就绪计时；角色选择成功后发送一次，C1353不再误发kind1/1220。build/vet/全量test与真实PVF巴卡尔专项通过。详细证据见bakal-recruitment-popup-review-20261010.md。仅冻结此次菜单弹窗修复，未扩大到竞拍、机制传送或其它UI数值；默认程序不因局部验收自动发布。


## 2026-10-11：基本战斗实机确认与远程合并保全

业主“我测试了基本没有问题现在就差最后的领奖阶段了”，并明确最终通关未出现结算窗口。上一轮交付52b3a1a9的基本战斗修复得到概括确认；不将最终结算/竞拍扩大为已完成。合并后75个相关文件一致，两处登录就绪/C16门禁接线丢失已恢复，远程新增功能保留。新候选包含结算stage3开窗修复，仍待实机。详见bakal-merge-and-result-review-20261011.md。
